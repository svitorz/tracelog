# Spec 0002: Endurecimento da API HTTP e correções de ingestão

- **Status:** Aprovada (decisões confirmadas pelo autor em 2026-11-20)
- **Autor:** Vitor Fábio
- **Data:** 2026-11-20
- **Escopo:** servidor HTTP, `POST /events`, `/health`, `/ready`, derivação de severidade, `metrics errors`, schema de `technical_event`
- **Relacionada:** [0001](./0001-ingestao-em-lote-idempotente.md) (esta spec implementa só a parte de deduplicação dela)

## 1. Problema

Verificado em execução na fase 0 (ver `PROGRESS.md`):

1. O servidor usa `http.ListenAndServe` sem timeouts e sem *graceful shutdown*.
2. O corpo é lido sem limite: um corpo de 20 MB foi aceito e gravado.
3. Todo erro vira `500` com a mensagem interna (SQL, constraint) no corpo da resposta.
4. A unicidade de `external_id` é global e a duplicata responde `500`.
5. `/health` responde `ok` a qualquer método e não diz nada sobre o banco.
6. A severidade é derivada por substring ("terror" vira `error`) e o cliente HTTP pode defini-la por `payload.level`.
7. `metrics errors` ignora `fatal` e agrupa por dia no fuso da sessão do PostgreSQL.
8. O corpo bruto é copiado em `payload._ingestion.raw_request_body` sempre que há qualquer aviso, duplicando o conteúdo.
9. IDs no formato UUID v6/v7/v8 são tratados como inválidos e caem no fallback.
10. Campos maiores que a coluna geram `500`.
11. `--to YYYY-MM-DD` exclui o próprio dia.

## 2. Objetivos

- Servidor com timeouts e encerramento gracioso.
- Nenhuma mensagem interna na resposta HTTP; detalhe só no log do servidor.
- Reenvio do mesmo evento (`application_id`, `external_id`) seguro e sem duplicata.
- Liveness e readiness separados.
- Severidade derivada só pelo servidor no canal HTTP, com heurística mais precisa e limites documentados.
- `metrics errors` mostrando `error` e `fatal`.

## 3. Não objetivos

- `POST /events/batch` e `external_id` obrigatório (spec 0001, D3 e D5): ficam para depois da v0.1.0.
- Autenticação, TLS, rate limit.
- Detecção de negação na heurística de severidade ("0 errors found" continua `error`).
- Mudanças no categorizador do `import logs` (fase 4).

## 4. Comportamento e contrato

### 4.1 Servidor

| Parâmetro | Valor | Motivo |
|-----------|-------|--------|
| `ReadHeaderTimeout` | 5s | Mitiga *slowloris*: cabeçalhos são pequenos. |
| `ReadTimeout` | 15s | Cabeçalho + corpo de até 1 MiB; exige ~70 KB/s do cliente. |
| `WriteTimeout` | 15s | Maior que o timeout do handler (10s), para a resposta de erro ainda sair. |
| `IdleTimeout` | 60s | Keep-alive sem prender conexões ociosas indefinidamente. |
| Timeout por requisição (contexto do handler) | 10s | Limita o tempo de banco por evento. |
| Timeout de shutdown | 8s | Abaixo dos 10s que o Docker espera antes do `SIGKILL`. |

`SIGINT` e `SIGTERM` param de aceitar conexões, esperam as requisições em andamento até o timeout de shutdown e então fecham o pool do banco.

Pool do banco: `MaxOpenConns=10`, `MaxIdleConns=10`, `ConnMaxLifetime=30m`, `ConnMaxIdleTime=5m`.

### 4.2 Rotas

| Rota | Métodos | Resposta |
|------|---------|----------|
| `GET /health` | `GET`, `HEAD` | `200 {"status":"ok"}`. Não toca o banco (liveness). |
| `GET /ready` | `GET`, `HEAD` | `200 {"status":"ready"}` se o ping no banco responder em até 2s; senão `503 {"status":"unavailable"}` (readiness). |
| `POST /events` | `POST` | Ver 4.3. |

Método não permitido: `405` com cabeçalho `Allow` e corpo `{"error":"method not allowed"}`.

### 4.3 `POST /events`

| Situação | Status | Corpo |
|----------|--------|-------|
| Evento novo | `201` | `{"id","external_id","received_at","created_at","duplicate":false}` |
| Já existe evento com o mesmo (`application_id`, `external_id`) | `200` | Campos do evento **existente**, `"duplicate": true` |
| `Content-Type` presente e diferente de `application/json` (ou `*/*+json`) | `415` | `{"error":"Content-Type must be application/json"}` |
| `Content-Type` ausente | segue como JSON | — |
| Corpo maior que `HTTP_MAX_BODY_BYTES` | `413` | `{"error":"request body exceeds N bytes"}` |
| JSON malformado ou corpo vazio | `201` | Evento gravado, como hoje (ingestão tolerante) |
| Qualquer falha interna | `500` | `{"error":"internal error"}`; o detalhe vai para o log |

`application_id` usado na deduplicação é o valor **já normalizado** (após o fallback). Eventos sem `external_id` não são deduplicados.

Não foram criados `400`, `409` e `422`: com a ingestão tolerante e a truncagem (4.5), nenhum caso atual os produz.

### 4.4 Severidade

- No canal `http`, `payload.level` **não** define a severidade (é só dado guardado). No canal `file`, o nível da linha de log continua valendo.
- A busca por palavras-chave na mensagem passa a casar **palavras inteiras**, com as flexões listadas no código (`crash`/`crashed`/`crashes`, `error`/`errors` etc.).
- Nomes de tipo em CamelCase terminados em `Error` ou `Exception` (`TypeError`, `IOException`) contam como erro.
- Limites conhecidos e documentados por teste: negação não é detectada ("0 errors found", "without failure" continuam `error`).

### 4.5 Normalização na ingestão

- `external_id` acima de 255 caracteres, `trace_id` e `correlation_id` acima de 128 são **truncados** e o fato é anotado em `payload._ingestion.normalization_warnings`. Risco aceito: dois `external_id` longos com o mesmo prefixo de 255 caracteres passam a ser tratados como o mesmo evento.
- `payload._ingestion.raw_request_body` só é gravado quando o corpo **não** pôde ser decodificado como JSON (único caso em que haveria perda de informação).
- Qualquer UUID no formato `8-4-4-4-12` hexadecimal é aceito como ID (antes só versões 1 a 5).

### 4.6 `metrics errors`

Saída:

```text
DATE        ERRORS  FATAL  TOTAL
----        ------  -----  -----
```

- Conta `severity IN ('error','fatal')`, por dia em **UTC** (`occurred_at AT TIME ZONE 'UTC'`).
- Dias sem `error` nem `fatal` continuam omitidos (preencher dias vazios é da camada de apresentação).
- Tabela e resumo vão para a saída padrão (stdout).

### 4.7 Filtros de período na CLI

`--to YYYY-MM-DD` passa a incluir o dia inteiro (até `23:59:59.999999999Z`). Com RFC3339 o valor é usado como está.

## 5. Configuração

| Variável | Padrão | Validação |
|----------|--------|-----------|
| `HTTP_MAX_BODY_BYTES` | `1048576` (1 MiB) | Inteiro positivo; senão a inicialização falha. |

## 6. Schema

`migrations/000003_external_id_per_application.sql` (idempotente), conforme spec 0001 D8: remove `uq_technical_event_external_id` e cria o índice único parcial `uq_technical_event_app_external_id (application_id, external_id) WHERE external_id IS NOT NULL`. O INSERT usa `ON CONFLICT ... DO NOTHING` e busca o existente (spec 0001 D9).

## 7. Critérios de aceite

1. `SIGTERM` com uma requisição em andamento: a requisição termina com sucesso e o processo sai com código 0.
2. Corpo acima do limite → `413`, nada gravado.
3. `Content-Type: text/plain` → `415`, nada gravado; sem `Content-Type` → aceito.
4. Nenhuma resposta `500` contém texto vindo do banco.
5. Mesmo (`application_id`, `external_id`) duas vezes → `201` e depois `200` com `duplicate: true` e o mesmo `id`; uma linha só. Apps diferentes com o mesmo `external_id` → dois eventos.
6. `/ready` → `503` com o banco fora; `/health` → `200`.
7. `{"payload":{"level":"fatal"},"message":"all good"}` via HTTP → `info`.
8. Testes de tabela documentam a heurística, inclusive os falsos positivos que permanecem.
9. `metrics errors` mostra `fatal` em coluna própria.
10. `go test ./...` passa sem banco; testes de integração rodam só com `TRACELOG_TEST_DATABASE_URL`.

## 8. Questões em aberto

Nenhuma bloqueante.
