# Spec 0001: Ingestão em lote e idempotente

- **Status:** Aprovada (decisões confirmadas pelo autor; ver seção 10)
- **Autor:** Vitor Fábio
- **Data:** 2026-10-15
- **Escopo:** interface HTTP de ingestão e schema de `technical_event`

## 1. Problema

Os três modos de implantação previstos têm a mesma necessidade de ingestão, mas com comportamentos diferentes:

- **Servidor local para um time:** apps enviam eventos em rajadas, com retries de rede.
- **Instalação por máquina:** o cliente pode ficar offline por horas e reenviar tudo ao reconectar.
- **Servidor regional para apps desktop:** muitos clientes com relógio e conectividade não confiáveis.

Hoje o projeto tem três problemas:

1. **Não há idempotência real.** A constraint `uq_technical_event_external_id` é `UNIQUE (external_id)` global. Quando um cliente reenvia um evento já recebido, o INSERT falha e o `POST /events` responde `500`. Para o cliente, um retry bem-sucedido parece falha permanente.
2. **A unicidade é global, não por aplicação.** Duas aplicações diferentes que usem o mesmo `external_id` (ex.: um contador `1`, `2`, ...) colidem entre si.
3. **Não há envio em lote.** Um cliente que tenha 500 eventos acumulados precisa fazer 500 requisições. Não há limite de tamanho do corpo, então um lote grande pode consumir toda a memória do servidor.

## 2. Objetivos

- Reenviar um evento já recebido **não pode criar duplicata** e **não pode gerar erro** para o cliente.
- Permitir enviar **vários eventos em uma requisição**.
- Fornecer **resultado por evento**, para que o cliente saiba exatamente o que foi aceito, o que já existia e o que falhou.
- Definir **limites explícitos** de tamanho para proteger o servidor.
- Garantir que todo evento HTTP seja deduplicável, exigindo `external_id` em `POST /events` e no lote.

## 3. Não objetivos

- Autenticação, autorização ou identificação de clientes (fora do MVP).
- Fila assíncrona, buffer no servidor ou backpressure. O servidor responde de forma síncrona.
- Idempotência para a importação de arquivos (`import logs`). Linhas de arquivo não têm `external_id`; isso pode ser tratado em outra spec.
- Transações all-or-nothing entre eventos do mesmo lote (ver decisão D4).
- Rate limiting.

## 4. Decisões propostas

Cada decisão tem uma recomendação. As que ainda não foram confirmadas estão marcadas em "Questões em aberto" (seção 10).

### D1. Chave de idempotência

A unicidade passa a ser **`(application_id, external_id)`**, considerando apenas eventos com `external_id` não nulo.

- `application_id` é o valor **já normalizado** pelo caso de uso (após o fallback, se houver).
- Eventos sem `external_id` não são deduplicados, como hoje.

### D2. Comportamento de `POST /events` (single)

| Situação | Status | Corpo |
|----------|--------|-------|
| Evento novo | `201 Created` | Igual ao atual, com `"duplicate": false` |
| Evento já existente para a mesma `(application_id, external_id)` | `200 OK` | Retorna o `id` e os campos do evento **existente**, com `"duplicate": true` |
| `external_id` ausente ou vazio | `400 Bad Request` | `{"error": "external_id is required"}`, nada é gravado |
| Falha de persistência | `500` | `{"error": "..."}`, como hoje |

Retornar `200` (e não `409`) torna o reenvio seguro: um cliente que não sabe se o primeiro envio chegou pode reenviar sem tratar erro.

### D3. Endpoint de lote

**`POST /events/batch`**

Requisição:

```json
{
  "events": [
    {
      "external_id": "app-42-000001",
      "message": "Database connection timeout",
      "occurred_at": "2026-03-09T14:30:00Z",
      "category": "system",
      "application_id": "<uuid>",
      "source_id": "<uuid>",
      "environment_id": "<uuid>",
      "payload": {"retry_count": 3}
    }
  ]
}
```

Os campos de cada evento são os mesmos de `POST /events`.

Resposta: **`200 OK`**, sempre que a requisição foi lida e validada como lote (mesmo que alguns itens tenham falhado):

```json
{
  "total": 3,
  "created": 1,
  "duplicates": 1,
  "rejected": 1,
  "results": [
    {"index": 0, "status": "created", "id": "<uuid>", "external_id": "app-42-000001"},
    {"index": 1, "status": "duplicate", "id": "<uuid>", "external_id": "app-42-000002"},
    {"index": 2, "status": "rejected", "external_id": "app-42-000003", "error": "external_id is required for batch ingestion", "retryable": false}
  ]
}
```

Estados de item:

| `status` | Significado | `retryable` |
|----------|-------------|-------------|
| `created` | Persistido agora | — |
| `duplicate` | Já existia com a mesma chave | — |
| `rejected` | Inválido (ex.: sem `external_id`, JSON de item inválido) | `false` |
| `error` | Falha transitória de persistência (ex.: banco indisponível) | `true` |

`results` mantém a ordem de `events` e o campo `index` referencia a posição original.

### D4. Semântica do lote: não atômica

Cada item é processado de forma independente. Um item rejeitado ou com erro **não cancela** os outros. O motivo: o cliente reenvia o lote inteiro ou só os itens com `retryable: true`, e a idempotência de D1 garante que os já gravados não sejam duplicados.

Transação única por lote fica como opção futura (ver seção 10).

### D5. `external_id` obrigatório em todos os eventos HTTP

Todo evento recebido por HTTP, tanto em `POST /events` quanto em cada item de `POST /events/batch`, precisa de `external_id`. Sem ele não há como reenviar com segurança, e a idempotência de D1 deixaria de valer.

- Em `POST /events`, ausência gera `400` (ver D2).
- Em `POST /events/batch`, o item sem `external_id` recebe `status: rejected` e os demais itens seguem normalmente.

A regra se aplica apenas ao canal HTTP. A importação de arquivos (`import logs`) e qualquer outro canal futuro não são afetados, pois linhas de log não têm `external_id`. Na implementação, a validação deve ficar no adaptador HTTP ou na entrada do caso de uso específica do canal HTTP, não em regra geral de `RegisterTechnicalEventUseCase`.

### D6. Limites

Novas variáveis de ambiente, com padrões:

| Variável | Padrão | Efeito quando excedido |
|----------|--------|------------------------|
| `HTTP_MAX_BODY_BYTES` | `1048576` (1 MiB) | `413 Payload Too Large` |
| `HTTP_MAX_BATCH_SIZE` | `500` | `400` com `{"error": "batch exceeds maximum size of 500 events"}` |

O corpo é lido com `http.MaxBytesReader`, não com `io.ReadAll` sem limite. Esse limite também se aplica a `POST /events`.

Esses valores precisam ser documentados em `.env.example` e no guia do projeto.

### D7. Erros da requisição (não de item)

| Código | Quando |
|--------|--------|
| `400` | JSON malformado, corpo sem campo `events`, lote vazio ou acima de `HTTP_MAX_BATCH_SIZE`, ou `external_id` ausente em `POST /events` |
| `405` | Método diferente de `POST` |
| `413` | Corpo acima de `HTTP_MAX_BODY_BYTES` |
| `500` | Apenas em `POST /events` (single) para falha de persistência |

Observação: hoje `POST /events` aceita JSON malformado e grava o evento com `payload._ingestion.decode_error`. Como não há clientes atuais, esse comportamento pode ser revisto, mas **não faz parte desta spec**: nesta entrega, JSON malformado continua tolerado no single (com a consequência de que, sem `external_id` decodificado, a requisição recebe `400`). Para o lote, JSON malformado no nível da requisição é `400`, pois não há como identificar os itens.

### D8. Schema

Migration nova, `migrations/000003_external_id_per_application.sql`, idempotente:

```sql
BEGIN;

ALTER TABLE technical_event
    DROP CONSTRAINT IF EXISTS uq_technical_event_external_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_technical_event_app_external_id
    ON technical_event (application_id, external_id)
    WHERE external_id IS NOT NULL;

COMMIT;
```

Essa migration é segura para dados existentes: a constraint global já garante que não há `external_id` repetido, então o índice por aplicação não falha ao ser criado.

### D9. Persistência com concorrência

O INSERT usa `ON CONFLICT (application_id, external_id) WHERE external_id IS NOT NULL DO NOTHING RETURNING ...`. Se não retornar linha, o repositório busca o evento existente pela mesma chave e o devolve com `duplicate = true`.

Isso evita a corrida em que duas requisições com o mesmo evento passam pela checagem ao mesmo tempo.

## 5. Mudanças por camada (arquitetura)

- **`core/domain`:** nenhuma mudança obrigatória. Pode-se adicionar um tipo de resultado de ingestão.
- **`core/ports`:** `TechnicalEventRepository.Save` passa a informar se o evento já existia (ex.: retorno `(event, created bool, err error)`), ou um tipo de resultado equivalente.
- **`core/services`:** `RegisterTechnicalEventUseCase` expõe o resultado de criação/duplicata. Um novo caso de uso de lote (`RegisterTechnicalEventBatchUseCase`) chama o existente por item, sem duplicar a lógica de normalização.
- **`adapters/repository`:** INSERT com `ON CONFLICT` e busca do existente.
- **`adapters/http`:** rota `/events/batch`, `http.MaxBytesReader`, DTOs de lote e mapeamento de status.
- **`config`:** `HTTP_MAX_BODY_BYTES` e `HTTP_MAX_BATCH_SIZE`, com validação (valores positivos).

## 6. Critérios de aceite

1. Enviar o mesmo `POST /events` (com `external_id`) duas vezes resulta em `201` na primeira e `200` com `"duplicate": true` na segunda, com o mesmo `id`. Existe apenas uma linha em `technical_event`.
2. Dois eventos com o mesmo `external_id` em **aplicações diferentes** são aceitos como eventos distintos.
3. `POST /events/batch` com 3 itens (1 novo, 1 já existente, 1 sem `external_id`) retorna `200` com `total=3`, `created=1`, `duplicates=1`, `rejected=1`, e `results` na ordem de entrada.
4. Um lote que excede `HTTP_MAX_BATCH_SIZE` retorna `400` e nada é gravado.
5. Um corpo acima de `HTTP_MAX_BODY_BYTES` retorna `413` e nada é gravado, em `/events` e em `/events/batch`.
6. JSON malformado em `/events/batch` retorna `400` e nada é gravado.
7. Método `GET` em `/events/batch` retorna `405`.
8. Migration `000003` pode ser executada duas vezes sem erro, em banco novo e em banco com a `000002` aplicada.
9. Testes de caso de uso (fakes) cobrem: criação, duplicata, rejeição por falta de `external_id`, erro transitório marcado como `retryable`, e ordem dos resultados.
10. `docs/guia-do-projeto.md` e `.env.example` são atualizados com a nova rota e as novas variáveis.
11. `POST /events` sem `external_id` retorna `400` e nada é gravado.
12. `HTTP_MAX_BODY_BYTES` e `HTTP_MAX_BATCH_SIZE` são lidos do ambiente. Valores não positivos são rejeitados na inicialização, com mensagem clara.
13. A importação de arquivos (`import logs`) continua funcionando sem `external_id`.

## 7. Testes necessários

- Unitários em `internal/core/services/` com fake do repositório (critérios 3, 9).
- Testes de handler com `httptest` para status e limites (critérios 4, 5, 6, 7).
- Teste de integração com PostgreSQL para o `ON CONFLICT` e para a migration (critérios 1, 2, 8). Como ainda não há infraestrutura de integração, esse teste precisa ser explicitamente opcional (ex.: variável `TRACELOG_TEST_DATABASE_URL`), para que `go test ./...` passe sem banco.

## 8. Compatibilidade

- Não há clientes em produção. Mudanças de contrato desta spec não exigem período de transição.
- `POST /events` passa a exigir `external_id`, e duplicatas passam a responder `200` (em vez de `500`), com o campo novo `duplicate`.
- Nenhum dado existente precisa ser alterado. Linhas já gravadas sem `external_id` continuam válidas (o índice é parcial).

## 9. Fora desta spec, mas relacionado

- Validação de tamanho por campo (ex.: `message` máxima). Pode virar spec própria.
- Métricas de ingestão (quantos itens aceitos, rejeitados, duplicados por minuto).

## 10. Decisões confirmadas

| # | Questão | Decisão |
|---|---------|---------|
| 1 | `external_id` obrigatório também no `POST /events`? | **Sim.** Não há clientes atuais, então o motivo para torná-lo opcional deixou de existir. Aplicado em D2 e D5. |
| 2 | Lote não atômico (D4)? | **Sim.** Cada item é independente no MVP. |
| 3 | Tamanho máximo por evento | **Em aberto**, em spec própria. Não bloqueia esta entrega. |
| 4 | Nomes `duplicate` e `/events/batch` | Mantidos como propostos, sem objeção. |
| 5 | Limites de 1 MiB e 500 itens | **Mantidos, mas configuráveis** via `HTTP_MAX_BODY_BYTES` e `HTTP_MAX_BATCH_SIZE` (D6). Os valores são padrões, não constantes no código, e serão reavaliados com dados reais de uso. |
