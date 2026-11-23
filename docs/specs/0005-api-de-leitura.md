# Spec 0005: API JSON de leitura

- **Status:** Aprovada (todas as recomendações da seção 8 e das questões em aberto confirmadas pelo autor em 2026-11-23)
- **Autor:** Vitor Fábio
- **Data:** 2026-11-23
- **Escopo:** novas rotas HTTP de leitura, erros tipados de leitura, paginação, endereço de escuta do `serve`, novos ports de leitura
- **Relacionadas:** [0002](./0002-endurecimento-da-api-http.md) (infra HTTP e mapeamento de erros que esta spec estende), [0004](./0004-importacao-idempotente-e-dados-de-exemplo.md) (dataset usado nos critérios de aceite). A interface web será a spec 0006.

## 1. Problema

Hoje a leitura só existe na CLI (`events list`, `metrics errors`, `db list`). O dashboard planejado (spec 0006) precisa de três telas, e nenhuma pode ser servida pelo HTTP atual:

| Tela | Precisa de | Situação hoje (verificado na Fase 0 da Trilha 2, `PROGRESS.md`) |
|------|------------|------------------------------------------------------------------|
| **Visão geral** | erros e fatais por dia (com dias vazios), contadores por severidade no período, um período padrão que mostre dados | `GetErrorFrequencyUseCase` omite dias vazios; não há contagem por severidade; não há como saber onde estão os dados |
| **Eventos** | lista filtrada e paginada; nomes de aplicação e ambiente para os filtros | só `Limit`, sem desempate na ordenação (`ORDER BY occurred_at DESC`) nem teto; dados de referência só por SQL direto em `cmd/db.go` |
| **Detalhe do evento** | um evento completo por ID | não há busca por ID |

Além disso:

1. Os erros de validação dos casos de uso são `fmt.Errorf` sem tipo. O adapter HTTP não consegue separar `400` de `500`.
2. O `serve` escuta em todas as interfaces (`:PORT`). Só o compose restringe a `127.0.0.1`. Uma API de leitura serve `payload`, `raw_line` e `_ingestion.raw_request_body`, que podem conter dados sensíveis e não têm redação.
3. O dataset de exemplo tem 14 dias em março de 2026, mas 51 dos 429 eventos (12%) não têm timestamp utilizável e recebem a hora do import como `occurred_at`. Um default como "últimos 7 dias" mostra uma tela vazia, e "todo o período" mostra de março até o dia do import.

## 2. Objetivos

- Servir as três telas com o menor conjunto de rotas: cada rota existe porque uma tela precisa dela.
- Validação estrita: parâmetro inválido, desconhecido ou fora da faixa responde `400` com uma mensagem útil e o nome do parâmetro. A tolerância da ingestão **não** se aplica à leitura.
- Paginação sem perder nem repetir linhas, inclusive com timestamps iguais.
- Nenhum erro interno vazando na resposta (regra da spec 0002).
- Leitura atrás de ports, com SQL só nos adapters, e casos de uso testados com fakes.
- Bind padrão em localhost, para que a leitura de dados não confiáveis não fique exposta na rede por acidente.

## 3. Não objetivos

- Autenticação, autorização, TLS, rate limit e redação de dados sensíveis. Continuam fora do MVP; a exposição é documentada (seção 4.8).
- Busca textual, filtro por `trace_id` ou `correlation_id`, agrupamento por fingerprint (item 1 do roadmap).
- Escrita pela API nova. `POST /events` não muda.
- Ordenação configurável, contagem total de resultados, exportação.
- Mudanças na CLI. `events list` e `metrics errors` mantêm a saída atual; a única mudança visível é que a ordem de eventos com o mesmo `occurred_at` passa a ser determinística (desempate por `id`).
- Dados de exemplo com várias aplicações (ver questão em aberto 2).

## 4. Comportamento

### 4.1 Rotas

Todas as rotas novas ficam sob `/api/v1` e aceitam `GET` e `HEAD`. Qualquer outro método responde `405` com `Allow: GET, HEAD`.

| Rota | Tela | Resumo |
|------|------|--------|
| `GET /api/v1/events` | Eventos | Lista paginada com filtros |
| `GET /api/v1/events/{id}` | Detalhe | Um evento completo |
| `GET /api/v1/metrics/error-frequency` | Visão geral | `error` e `fatal` por dia UTC, dias vazios preenchidos |
| `GET /api/v1/metrics/severity-counts` | Visão geral | Contagem por severidade no período |
| `GET /api/v1/metrics/data-range` | Visão geral | Onde estão os dados, mais o período padrão sugerido |
| `GET /api/v1/applications` | Filtros, Detalhe | Aplicações monitoradas |
| `GET /api/v1/environments` | Filtros, Detalhe | Ambientes |
| `GET /api/v1/sources` | Detalhe | Fontes, para mostrar nome em vez de UUID |

As rotas existentes (`/health`, `/ready`, `POST /events`) não mudam. Qualquer caminho desconhecido sob `/api/` responde `404` em JSON (seção 5.2). Fora de `/api/`, o comportamento do `net/http` continua até a spec 0006 decidir como a UI é servida.

### 4.2 Parâmetros de consulta comuns

| Parâmetro | Formato | Regras |
|-----------|---------|--------|
| `application_id`, `environment_id` | UUID `8-4-4-4-12` hexadecimal, qualquer versão | Inválido → `400`. Válido, mas inexistente → resultado vazio (é um filtro, não uma busca por recurso). |
| `from` | RFC3339 (com ou sem fração, com offset) ou `YYYY-MM-DD` | `YYYY-MM-DD` vale 00:00:00 UTC. Inclusivo. |
| `to` | igual a `from` | `YYYY-MM-DD` vale o dia inteiro (até 23:59:59.999999999 UTC), como na CLI (spec 0002 §4.7). Inclusivo. |

- `from` posterior a `to` → `400`.
- Um parâmetro desconhecido → `400` (pega erros de digitação como `severty=error`). Um parâmetro repetido (`severity=a&severity=b`) → `400`. Um valor vazio (`severity=`) equivale a ausente.
- Os parâmetros que cada rota aceita estão na seção 5.1. Um parâmetro válido em outra rota, mas não nesta, conta como desconhecido.

### 4.3 Lista de eventos e paginação

- Ordem fixa: `occurred_at DESC, id DESC`. O `id` desempata eventos com o mesmo `occurred_at`: no volume `-scale 200` há 4.638 timestamps repetidos.
- `limit`: inteiro de 1 a 200; padrão 50 (o mesmo da CLI). Fora da faixa ou não inteiro → `400`. O valor nunca é ajustado em silêncio.
- **Paginação por cursor (keyset).** `next_cursor` é opaco para o cliente: base64url, sem padding, de `{"t":"<occurred_at RFC3339Nano>","id":"<uuid>"}` do último item da página. A próxima página é `occurred_at, id` estritamente menor que o cursor, com os mesmos filtros reenviados pelo cliente.
- O repositório busca `limit + 1` linhas. Se vier a linha extra, `next_cursor` é preenchido; senão é `null` (última página).
- Cursor que não decodifica, com JSON inválido, `t` fora de RFC3339 ou `id` que não é UUID → `400` com código `invalid_cursor`. O cursor não é assinado: ele só carrega uma posição, que não é segredo, e um cursor adulterado mas bem formado apenas posiciona a lista em outro ponto. Trocar os filtros mantendo o cursor é permitido: a lista continua daquela posição com os filtros novos.
- Os itens da lista **não** trazem `payload`, `raw_line` nem `parse_error` (seção 4.6).
- Garantia: eventos gravados durante a navegação com `occurred_at` mais novo que a primeira página não deslocam as páginas seguintes. Um evento gravado com `occurred_at` antigo, por exemplo num import, pode aparecer numa página ainda não visitada. Isso é aceito.

### 4.4 Frequência de erros

- Mesma contagem da CLI: `severity IN ('error','fatal')`, por dia UTC.
- **`from` e `to` são obrigatórios** e os dias vazios são preenchidos com zeros, de `from` até `to` inclusive, no caso de uso Go, em UTC. Sem um deles → `400`.
- Intervalo máximo de 366 dias. Acima disso → `400` (limita o tamanho da resposta e o trabalho do preenchimento).
- `from`/`to` com hora são truncados para o dia UTC **só na lista de dias**; a contagem usa os instantes exatos, como a CLI.
- O `GetErrorFrequencyUseCase` atual não muda (a CLI continua omitindo dias vazios, spec 0002 §4.6). O preenchimento é uma função de serviço nova, usada pela rota HTTP.

### 4.5 Contadores por severidade e intervalo de dados

- `severity-counts`: aceita os filtros comuns (`application_id`, `environment_id`, `from`, `to`, todos opcionais) e devolve as cinco severidades, sempre presentes, com zero quando não houver eventos, mais o `total`.
- `data-range`: sem parâmetros. Devolve:
  - `first_occurred_at` e `last_occurred_at`: menor e maior `occurred_at` da tabela, ou `null` sem eventos.
  - `last_error_at`: maior `occurred_at` com `error` ou `fatal`, ou `null`.
  - `default_from` e `default_to` (`YYYY-MM-DD`): **14 dias UTC terminando no dia de `last_error_at`**; sem nenhum erro, terminando no dia de `last_occurred_at`; sem nenhum evento, terminando hoje (UTC). A regra fica num caso de uso Go, testado com fakes e relógio injetado.
- Com o dataset de exemplo, `default_from` = `2026-03-01` e `default_to` = `2026-03-14`, independentemente da data do import: os 51 eventos sem timestamp são todos `info` ou `warn` (medido na Fase 0).

### 4.6 Campos expostos

| Campo | Lista | Detalhe | Observação |
|-------|:-----:|:-------:|------------|
| `id`, `occurred_at`, `received_at`, `created_at` | sim | sim | |
| `severity`, `category`, `normalization_status`, `ingestion_channel` | sim | sim | |
| `message` | sim | sim | Completa. O corte visual é da UI. |
| `application_id`, `environment_id`, `source_id` | sim | sim | Só IDs. Os nomes vêm das rotas de referência (sem JOIN). |
| `external_id`, `trace_id`, `correlation_id` | sim | sim | `null` quando ausentes |
| `parse_error`, `raw_line` | não | sim | `null` quando ausentes |
| `payload` (com `_ingestion`, inclusive `raw_request_body`) | não | sim | Objeto como gravado. |

- Strings opcionais ausentes saem como `null`, não `""`: o cliente distingue ausência de vazio sem convenção extra. Os horários saem em UTC, RFC3339 com fração quando houver (`time.Time` do Go). As datas do gráfico saem como `YYYY-MM-DD`.
- Os campos de evento usam snake_case, como as respostas atuais (R6).

### 4.7 Endereço de escuta

- Variável nova `HTTP_HOST`, padrão `127.0.0.1`. O `serve` passa a escutar em `HTTP_HOST:HTTP_PORT`.
- No compose, o serviço `api` recebe `HTTP_HOST=0.0.0.0` (é necessário dentro do container para a porta publicada funcionar). A publicação continua só em `127.0.0.1`.
- A imagem também define `ENV HTTP_HOST=0.0.0.0` (acrescentado na implementação, 2026-11-23): sem isso, `docker run -p` fora do compose publicaria uma porta inalcançável.
- **Mudança incompatível:** quem roda `tracelog serve` no host e recebe eventos de outras máquinas precisa definir `HTTP_HOST=0.0.0.0`. Vai no CHANGELOG como *Changed* com aviso.
- Validação: um valor que não é IP nem nome de host válido faz a inicialização falhar, como `HTTP_PORT`.

### 4.8 Segurança da leitura

- Sem CORS: nenhuma rota envia `Access-Control-Allow-Origin`. Em desenvolvimento, a UI usa o proxy do Vite; em produção, a mesma origem (decisão da spec 0006). Navegadores bloqueiam a leitura de outra origem por padrão.
- Todas as respostas de `/api/` levam `Cache-Control: no-store` e `X-Content-Type-Options: nosniff`.
- O JSON é gerado por `encoding/json` com o escape de HTML padrão (`<` vira `\u003c`), o que é defesa em profundidade e não substitui a renderização como texto na UI (regra de segurança do front na spec 0006).
- `payload`, `message` e `raw_line` são entrada não confiável, gravada como veio. A API não os interpreta nem os sanitiza: só serializa.
- Risco aceito e documentado: quem alcança a porta lê tudo, inclusive dados sensíveis que um cliente tenha enviado. O bind em localhost é a mitigação; não é controle de acesso.

### 4.9 Arquitetura

- **Erros tipados:**
  - `ports.ErrNotFound`, devolvido pelo repositório na busca por ID.
  - `services.ValidationError{Field, Message}`, devolvido pelos casos de uso de leitura para entrada inválida, com `errors.As` no adapter.
  - `ports.ErrReferenceNotFound` (spec 0002) continua só na ingestão.
- **Ports:**
  - `TechnicalEventRepository` ganha `FindByID` e um campo `After *EventCursor` (com `OccurredAt`, `ID`) em `TechnicalEventFilters`. A ordenação passa a desempatar por `id`.
  - Port novo `EventStatsRepository`, com `CountBySeverity` e `DataRange`.
  - Port novo `ReferenceDataRepository`, com `ListApplications`, `ListEnvironments` e `ListSources`. O `cmd/db.go` passará a usá-lo, num commit separado e com OK do autor.
- **Parsing:** o parsing dos parâmetros e do cursor fica no adapter HTTP. A validação de regra (faixa de `limit`, `from <= to`, intervalo máximo, severidade válida) fica nos casos de uso, para valer igual para qualquer adapter.
- **Handlers:** só traduzem query string → input → caso de uso → DTO. O mapeamento de erros segue a spec 0002: tipado → `4xx` com a mensagem do erro tipado; qualquer outro → `500 internal error`, com o detalhe só no log.
- **Timeout:** 10s por requisição, o mesmo de `POST /events`.

## 5. Contrato

### 5.1 Rotas

**`GET /api/v1/events`**: parâmetros `severity` (`debug|info|warn|error|fatal`), `application_id`, `environment_id`, `from`, `to`, `limit`, `cursor`.

```json
{
  "items": [
    {
      "id": "0f3c2a9e-1b7d-4c55-9a7e-2d8f1e6b4a10",
      "external_id": "file:3b6f0c1e9a7d4b2f8c5e1a0d9b7c6e4f",
      "occurred_at": "2026-03-10T14:47:00Z",
      "received_at": "2026-11-23T20:38:40.512345Z",
      "created_at": "2026-11-23T20:38:40.514001Z",
      "severity": "fatal",
      "category": "system",
      "message": "order worker crashed: connection pool exhausted",
      "normalization_status": "normalized",
      "ingestion_channel": "file",
      "trace_id": "9c1d7e0a4b3f2e18",
      "correlation_id": null,
      "application_id": "192524eb-8a13-4ac4-afe1-42850aa716a5",
      "environment_id": "6b6eae04-cf16-4999-9ec5-b805528e905b",
      "source_id": "5d2b8c71-0e4a-4f9b-b3c6-7a1e9d8f2c40"
    }
  ],
  "next_cursor": "eyJ0IjoiMjAyNi0wMy0xMFQxNDo0NzowMFoiLCJpZCI6IjBmM2MyYTllLTFiN2QtNGM1NS05YTdlLTJkOGYxZTZiNGExMCJ9"
}
```

**`GET /api/v1/events/{id}`**: sem parâmetros de consulta. O corpo tem todos os campos do item da lista, mais:

```json
{
  "parse_error": null,
  "raw_line": "{\"level\":\"fatal\",\"message\":\"order worker crashed: connection pool exhausted\",\"timestamp\":\"2026-03-10T14:47:00Z\"}",
  "payload": {
    "level": "fatal",
    "_ingestion": { "normalization_warnings": ["application_id missing; used default setup"] }
  }
}
```

**`GET /api/v1/metrics/error-frequency`**: parâmetros `from` (obrigatório), `to` (obrigatório), `application_id`, `environment_id`.

```json
{
  "from": "2026-03-01",
  "to": "2026-03-04",
  "days": [
    { "date": "2026-03-01", "errors": 2, "fatal": 0, "total": 2 },
    { "date": "2026-03-02", "errors": 2, "fatal": 0, "total": 2 },
    { "date": "2026-03-03", "errors": 1, "fatal": 0, "total": 1 },
    { "date": "2026-03-04", "errors": 0, "fatal": 0, "total": 0 }
  ]
}
```

**`GET /api/v1/metrics/severity-counts`**: parâmetros `application_id`, `environment_id`, `from`, `to`.

```json
{ "counts": { "debug": 0, "info": 266, "warn": 73, "error": 38, "fatal": 1 }, "total": 378 }
```

**`GET /api/v1/metrics/data-range`**: sem parâmetros.

```json
{
  "first_occurred_at": "2026-03-01T08:46:09Z",
  "last_occurred_at": "2026-11-23T20:38:40.620158Z",
  "last_error_at": "2026-03-14T21:12:00Z",
  "default_from": "2026-03-01",
  "default_to": "2026-03-14"
}
```

**`GET /api/v1/applications`**, **`/environments`**, **`/sources`**: sem parâmetros, sem paginação (são tabelas de setup, com poucas linhas). Ordem: aplicações por `name`, ambientes por `code`, fontes por `name` (a mesma de `db list`).

```json
{ "items": [ { "id": "192524eb-8a13-4ac4-afe1-42850aa716a5", "name": "Local Application", "service_key": "local-app", "description": null, "created_at": "2026-11-23T20:38:39.101112Z" } ] }
```

Ambiente: `id`, `code`, `name`, `description`, `created_at`. Fonte: `id`, `name`, `source_type`, `description`, `created_at`.

Os exemplos desta seção são **ilustrativos** (IDs, horários e contagens de `severity-counts` não são capturas). As fixtures reais serão geradas na fase 2 a partir da saída dos handlers (seção 5.4).

### 5.2 Erros

Formato de erro da leitura: o mesmo objeto da spec 0002, com dois campos a mais. O cliente atual de `POST /events` não é afetado, porque a ingestão continua com só `error`.

```json
{ "error": "limit must be an integer between 1 and 200", "code": "invalid_parameter", "field": "limit" }
```

| Status | `code` | Quando |
|--------|--------|--------|
| `400` | `invalid_parameter` | Valor inválido, fora da faixa, `from > to`, intervalo acima de 366 dias, obrigatório ausente, desconhecido ou repetido. `field` = nome do parâmetro. |
| `400` | `invalid_cursor` | Cursor malformado. `field` = `cursor`. |
| `404` | `not_found` | Evento inexistente em `/events/{id}`, ou caminho desconhecido sob `/api/`. Sem `field`. |
| `405` | `method_not_allowed` | Método diferente de `GET`/`HEAD`, com `Allow`. |
| `500` | `internal` | Qualquer falha não tipada. Mensagem fixa `internal error`. |

`/events/{id}` com um ID que não é UUID → `400` (`field` = `id`), não `404`: é erro do cliente, e evita mandar ao banco um valor que o PostgreSQL recusaria com erro interno (o que hoje acontece na CLI, guia §5.2).

### 5.3 Configuração

| Variável | Padrão | Validação |
|----------|--------|-----------|
| `HTTP_HOST` | `127.0.0.1` | IP ou nome de host; senão a inicialização falha. O compose define `0.0.0.0` no container. |

Documentada no `.env.example` e no guia §3.1.

### 5.4 Fixtures compartilhadas

Recomendação: **sim, com escopo pequeno.** Arquivos JSON em `internal/adapters/http/testdata/api/`, um por resposta representativa:
- lista com `next_cursor`;
- última página;
- detalhe com `_ingestion` e uma mensagem contendo `<script>`;
- frequência com dias preenchidos;
- contadores;
- data-range vazio e cheio;
- referência;
- um erro de cada código.

- No Go, os testes de handler comparam a resposta real (com fake determinístico) byte a byte com a fixture. Com `-update`, regravam.
- No front (spec 0006), os mocks e os testes de tipos importam os mesmos arquivos.
- **Ganho:** se o Go mudar um campo, o teste Go quebra; se o tipo TypeScript escrito à mão divergir, o teste do front quebra. A deriva fica visível dos dois lados sem gerador de tipos.
- **Custo:** o front passa a ler um caminho dentro da árvore Go (acoplamento de diretório), e as fixtures golden quebram em qualquer mudança cosmética de saída, que exige regravar conscientemente.
- **Descartado:** gerar tipos TS de um OpenAPI (ferramenta e dependência novas, e o OpenAPI viraria uma segunda fonte do contrato); manter mocks separados (deriva silenciosa).

## 6. Mudanças de schema

Nenhuma prevista. Um índice só entra na fase 2 se a medição `EXPLAIN (ANALYZE, BUFFERS)` com `loggen -scale 200` mostrar ganho em consulta usada por uma tela. Nesse caso entra por migration nova, idempotente, com o custo de escrita medido no import. Referência da Fase 0, com cache quente:
- primeira página sem filtro: 0,04 ms;
- página por cursor: 0,18 ms (o índice de coluna única + *Incremental Sort* já atende o desempate);
- `severity=error`: 6 ms, descartando 26.913 linhas (candidato a `(severity, occurred_at)`);
- frequência: 2,9 ms;
- contagem por severidade: 30 ms (Seq Scan).

## 7. Critérios de aceite

1. Cada rota da seção 5.1 responde como especificado contra o dataset de exemplo importado. Os comandos `curl` do guia §5.1 são executados e a saída colada é real.
2. Paginação: um teste de integração com N eventos de `occurred_at` idêntico, percorrido com `limit` pequeno, devolve os N eventos, cada um exatamente uma vez. A última página tem `next_cursor: null`. Uma página exatamente cheia no fim não gera página vazia extra.
3. Cursor adulterado (base64 inválido, JSON inválido, `t` ou `id` inválidos) → `400 invalid_cursor`.
4. `limit=0`, `limit=201`, `limit=abc`, `severity=critical`, `application_id=xyz`, `from=2026-13-01`, `from > to`, parâmetro desconhecido e parâmetro repetido → `400 invalid_parameter` com o `field` correto.
5. `/api/v1/events/{uuid inexistente}` → `404 not_found`; `/api/v1/events/abc` → `400`; `/api/v1/nada` → `404` em JSON; `POST /api/v1/events` → `405` com `Allow: GET, HEAD`.
6. Banco fora → `500` com `{"error":"internal error","code":"internal"}`, sem texto do banco.
7. `error-frequency` de `2026-03-01` a `2026-03-14` no dataset de exemplo: 14 dias, inclusive `2026-03-04` com zeros, e `2026-03-10` com `errors: 25, fatal: 1`.
8. `data-range` no dataset de exemplo: `default_from` = `2026-03-01`, `default_to` = `2026-03-14`. Os casos sem erros e sem eventos são cobertos por teste de caso de uso.
9. O detalhe de um evento com `message` `<script>alert(1)</script>` devolve o texto com `\u003c` e `Content-Type: application/json`, mais os cabeçalhos da seção 4.8.
10. O `serve` sem `HTTP_HOST` escuta só em `127.0.0.1` (verificado com `ss -ltn`). `make demo` continua funcionando, com a API alcançável pela porta publicada.
11. Os casos de uso novos têm testes com fakes; os handlers, com `httptest`; o repositório, testes de integração com `TRACELOG_TEST_DATABASE_URL` (`SKIP` sem banco).
12. As medições `EXPLAIN (ANALYZE, BUFFERS)` da lista (primeira página, página profunda por cursor, com filtro de severidade) e da frequência, com `loggen -scale 200`, ficam registradas no `PROGRESS.md` com método e limitações, com ou sem índice novo.
13. `go vet`, `gofmt` e `go test -race` passam; o CI fica verde.

## 8. Decisões para o autor

Cada item traz a recomendação primeiro.

**(a) Prefixo.** `/api/v1` para toda a leitura; `POST /events` fica como está, sem espelho.
- Por quê: a leitura vira um namespace isolado, sem colidir com as rotas do cliente da SPA (R8), e um `v1` custa três caracteres agora, enquanto renomear depois quebraria scripts e o MCP planejado (item 5 do roadmap).
- Descartados:
  - `/api` sem versão: mais simples, e o front é versionado junto com a API. O versionamento só serve a clientes externos, que ainda não existem. É a segunda melhor opção; aceito se você preferir.
  - Espelhar `POST /api/v1/events`: dois caminhos para o mesmo contrato, sem cliente que precise disso agora.
- Consequência para a 0006: `GET /events` continua `405`, então a UI não pode usar `/events` como rota de cliente no mesmo binário, ou o reload da página daria `405`.

**(b) Paginação.** Keyset `(occurred_at, id)` com cursor opaco.
- Por quê: estável sob inserções e com custo constante. Medido: 0,18 ms por página contra 43 ms com `OFFSET 40000` e Sort em disco.
- Custo: não há "ir para a página 37" nem total de páginas, e a navegação para trás depende do histórico do cliente (a pilha de cursores fica na URL ou no estado da UI; spec 0006).
- Descartado: offset, que é simples e permite saltos, mas repete ou pula linhas quando chegam eventos novos e piora linearmente.

**(c) Dias vazios.** Preenchidos no Go (caso de uso, UTC), com `from`/`to` obrigatórios e teto de 366 dias.
- Por quê: testável com fakes, o mesmo para qualquer cliente (MCP), e evita aritmética de datas no navegador, onde `Date` usa o fuso local e vira armadilha clássica de off-by-one.
- Descartado: preencher no front (resposta menor, API igual à CLI, mas lógica de datas duplicada e mais frágil).

**(d) Campos.** A lista vai sem `payload`, `raw_line` e `parse_error`; o detalhe vai completo, inclusive `_ingestion.raw_request_body`.
- Por quê: a tela de detalhe existe justamente para investigar o que chegou; esconder o payload a esvaziaria, e não existe redação. A lista sem payload limita o tamanho da resposta (até 1 MiB por evento × 200) e o volume de dado sensível trafegado por padrão.
- Descartados:
  - `?include=payload` na lista: parâmetro a mais sem tela que precise dele.
  - Omitir `raw_request_body` no detalhe: perde a única cópia do corpo que não decodificou.

**(e) Exposição e CORS.** `HTTP_HOST` com padrão `127.0.0.1`, e o compose define `0.0.0.0` no container. Sem middleware CORS: proxy do Vite em dev e mesma origem em produção.
- Por quê: a leitura expõe payloads, e a escuta em todas as interfaces deixa de ser um detalhe. O bind em localhost é a mitigação mais barata. Sem CORS não há configuração de origens a errar.
- Custo: mudança incompatível para quem usa `serve` no host recebendo da rede (precisa de `HTTP_HOST=0.0.0.0`).
- Descartados:
  - Manter `:PORT` e documentar: zero código, mas inseguro por padrão.
  - Middleware CORS com lista de origens: mais código e configuração, só necessário se a UI ficar em outra origem (opção (b) da spec 0006).

**(f) Teto, erros e códigos.** `limit` de 1 a 200, padrão 50, com `400` fora da faixa (nunca ajustado em silêncio). Parâmetro desconhecido ou repetido também dá `400`. O erro leva `error` + `code` + `field`. Os erros são tipados (`ValidationError` em services, `ErrNotFound` em ports).
- Por quê:
  - 200 itens sem payload cabem numa página de tabela, e com folga.
  - Rejeitar em vez de ajustar faz o cliente perceber o erro.
  - `code` permite tratar o erro no front sem casar texto.
  - Os erros tipados são o primeiro caso real de 400/404, que a spec 0002 não criou por não haver uso.
- Descartados:
  - Ajustar o `limit` ao teto (*clamp*): esconde o bug do cliente.
  - Ignorar parâmetros desconhecidos: tolerante, mas um filtro digitado errado devolve dados não filtrados sem aviso.
  - Adotar o RFC 9457 (`application/problem+json`): é padrão, mas muda o formato de erro já usado pela ingestão.

**(g) Período padrão.** `data-range` devolve `default_from`/`default_to`: 14 dias UTC terminando no dia do erro mais recente (ou do evento mais recente, ou hoje).
- Por quê: com o dataset de exemplo dá exatamente março 1–14. Num sistema ativo dá "últimos 14 dias". E não é enganado pelos 51 eventos sem timestamp, que caem na data do import.
- Descartados:
  - "Últimos 7 dias": tela vazia com o dataset.
  - "Todo o período": março a novembro, 268 dias quase vazios.
  - "14 dias até o evento mais recente": com o dataset, mostra o dia do import com 50 infos e nenhum erro.
  - Calcular no front: a regra ficaria em TypeScript, sem os testes de caso de uso.
- Custo: o "erro mais recente" pode ser um outlier antigo num banco quase só de info. A UI mostra o período e deixa trocar.

**(h) Índice.** Nenhum agora. Medir na fase 2 e só propor com ganho medido em consulta de tela, informando o custo no import.
- Candidato: `(severity, occurred_at DESC, id DESC)`, que atenderia a lista filtrada por severidade e a frequência.

**(i) Antecipação do roadmap.** Registrada no `PROGRESS.md` (log de decisões da Trilha 2).
- Custo concreto: as rotas novas vão ganhar SQL PostgreSQL a mais (cerca de 8 consultas novas) para reescrever quando vier o SQLite (item 2). A leitura fica atrás de ports exatamente para que essa reescrita fique no adapter.
- O núcleo de investigação (item 1: busca, `trace_id`, fingerprint) continua sem data, e a UI nasce sem essas visões.

**(j) Dados de referência.** Três rotas pequenas (`applications`, `environments`, `sources`) em vez de uma rota agregada.
- Por quê: cada uma tem cache próprio no front e corresponde a um método do port que o `cmd/db.go` também vai usar.
- Descartado: `GET /api/v1/reference` com as três listas (uma requisição só, mas um contrato menos óbvio). A diferença é pequena; aceito qualquer uma.

## 9. Questões em aberto

Resolvidas em 2026-11-23 com as recomendações abaixo: (1) sem severidade múltipla no MVP; (2) dataset com uma aplicação fica como está nesta spec e volta a ser decidido antes da fase 6; (3) cursor reaproveitável com filtros diferentes.

1. **Severidade múltipla na lista** (`severity=error,fatal`). Recomendação: não no MVP, para manter paridade com a CLI e com o port atual. Se a tela de Eventos pedir na fase 5, a mudança é aditiva (`= ANY`) e entra por atualização desta spec.
2. **O dataset de exemplo tem uma aplicação só.** `checkout-api`, `auth-service` e `legacy-gateway` são nomes de arquivo; o import grava tudo na aplicação do `init`. Os filtros de aplicação e ambiente funcionam, mas não demonstram nada. Opções, todas fora desta spec:
   - flag `--application-key` no `import logs` (mudança de CLI, precisa de spec);
   - a demo enviar eventos por HTTP com outras aplicações;
   - aceitar como está.
   
   Recomendação: decidir antes da fase 6 (demo); registrado em "Achados fora de escopo".
3. **Cursor e mudança de filtros.** Esta spec permite reaproveitar o cursor com filtros diferentes (seção 4.3). A alternativa seria embutir um hash dos filtros no cursor e responder `400` quando divergirem: mais rígido, mas sem caso real que justifique agora.
