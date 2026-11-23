# Guia do Projeto TraceLog

Referência única do estado atual do TraceLog: como instalar, executar, testar e estender, quais regras o código aplica e o que ele não faz. As decisões e o "porquê" de cada mudança estão nas specs de [`docs/specs/`](./specs/); as regras para quem contribui estão no [`AGENTS.md`](../AGENTS.md).

> Conferido contra o código e a execução real na fase 6 (2026-11-20). Blocos de saída marcados como "saída real" foram capturados de execuções, não escritos à mão. Se algo aqui divergir do código, o código prevalece: corrija este guia.

---

## 1. Visão geral

TraceLog é um MVP de observabilidade escrito em Go com PostgreSQL. Ele:

- recebe eventos técnicos de aplicações via HTTP (`POST /events`);
- importa eventos de arquivos de log em três formatos (`tracelog import logs`);
- normaliza o que chega, deriva a severidade no servidor e grava tudo em PostgreSQL;
- oferece uma CLI para listar eventos, contar erros por dia e inspecionar o setup;
- expõe uma API JSON de leitura em `/api/v1` (lista paginada, detalhe, frequência de erros, contadores, dados de referência; spec 0005).

**O que ele não faz:** não tem dashboard nem interface web (planejados na spec 0006), autenticação, TLS, rate limit, alertas, busca textual, agrupamento por fingerprint nem visão por `trace_id`. Não há lote no HTTP (`POST /events/batch`). Esses itens estão no roadmap do `AGENTS.md` ou nas specs como trabalho futuro.

### Decisões de comportamento que importam

- **A ingestão nunca descarta um evento que possa ser gravado.** Campos ausentes ou inválidos são normalizados e anotados em `payload._ingestion`; JSON malformado também vira evento. O HTTP só recusa quando não há como gravar: corpo grande demais (`413`), `Content-Type` que não é JSON (`415`) ou falha interna (`500`).
- **A severidade é derivada pelo servidor**, por heurística sobre a mensagem e o payload (seção 7.3). O cliente HTTP não a define.
- **Reenviar um evento é seguro:** mesmo `external_id` na mesma aplicação responde `200` com o evento já gravado. Reimportar um arquivo também não duplica.
- **`serve` e `import logs` exigem o setup** criado por `tracelog init`.

---

## 2. Pré-requisitos

| Ferramenta | Versão | Observação |
|------------|--------|------------|
| Docker + Docker Compose | plugin `docker compose` | Suficiente para subir tudo e rodar a demo. |
| Go | **1.26.11** (`go.mod`) | Só para rodar a CLI no host, testar ou compilar. Com `GOTOOLCHAIN=auto` (padrão), um Go mais antigo baixa essa versão sozinho. |
| `make`, `curl` | qualquer | Atalhos do `Makefile` e da demo. |

---

## 3. Instalação e configuração

Rode os comandos a partir da **raiz do repositório**: o `.env` é lido do diretório atual.

### 3.1 `.env`

```bash
cp .env.example .env
```

| Variável | Obrigatória | Padrão | Descrição |
|----------|-------------|--------|-----------|
| `APP_NAME` | não | `tracelog` | Nome exibido no log do `serve`. |
| `APP_ENV` | não | `local` | Ambiente exibido no log do `serve`. |
| `HTTP_HOST` | não | `127.0.0.1` | Endereço em que o `serve` escuta. Use `0.0.0.0` para receber de outras máquinas. A imagem e o compose usam `0.0.0.0` dentro do container. |
| `HTTP_PORT` | não | `8080` | Porta da API; o compose a publica em `127.0.0.1`. |
| `HTTP_MAX_BODY_BYTES` | não | `1048576` | Tamanho máximo do corpo de `POST /events`. Acima disso: `413`. Precisa ser positivo. |
| `DB_HOST` | não | `localhost` | Host do PostgreSQL para a CLI no host (o container usa `postgres`). |
| `DB_PORT` | não | `5432` | Porta do PostgreSQL para a CLI no host; o compose a publica em `127.0.0.1`. |
| `DB_NAME` | **sim** | — | Nome do banco. |
| `DB_USER` | **sim** | — | Usuário do banco. |
| `DB_PASSWORD` | **sim** | — | Senha. Pode ter espaços e aspas (o DSN escapa os valores). |
| `DB_SSLMODE` | não | `disable` | `disable`, `allow`, `prefer`, `require`, `verify-ca` ou `verify-full`. |

- **Precedência:** variável de ambiente do shell > `.env`.
- Sem `DB_NAME`, `DB_USER` e `DB_PASSWORD`, **qualquer** comando falha antes de executar, inclusive `--help`.
- O parser do `.env` é simples: não use comentário no fim da linha (`KEY=valor # x` faz o `# x` entrar no valor).

### 3.2 Subir com Docker Compose

```bash
docker compose up -d --wait     # ou: make up
docker compose ps -a
```

| Serviço | Faz | Começa quando |
|---------|-----|---------------|
| `postgres` | `postgres:16.16-alpine`, volume `tracelog_postgres_data`, healthcheck `pg_isready` | — |
| `migrate` | `tracelog migrate` e termina | `postgres` saudável |
| `init` | `tracelog init --defaults` e termina | `migrate` terminou com código 0 |
| `api` | `tracelog serve`, healthcheck em `/ready` | `init` terminou com código 0 |

`migrate` e `init` aparecem como `Exited (0)`: é o esperado. Rodar `up` de novo não reaplica migrations nem muda IDs. A imagem `tracelog:local` vem do `Dockerfile` (multi-stage, `alpine:3.24`, processo com UID 10001).

- Dentro do compose a aplicação usa `DB_HOST=postgres` e `DB_PORT=5432`; o `DB_HOST`/`DB_PORT` do `.env` valem só para a CLI no host.
- Banco e API são publicados **só em `127.0.0.1`**. Se a porta já estiver em uso, troque `DB_PORT` ou `HTTP_PORT` no `.env`.
- Só o banco, para usar a CLI no host: `docker compose up -d postgres`.
- Apagar os dados: `docker compose down -v`.

**O que não é seguro neste setup:** a API não tem autenticação (quem alcança a porta grava eventos e **lê todos eles**, inclusive `payload`, `raw_line` e o corpo bruto em `_ingestion`, sem nenhuma redação de dado sensível), não há TLS (nem cliente→API nem API→banco), a senha padrão do banco é `postgres` e não há rate limit. Publicar só em `127.0.0.1` reduz a exposição, mas não resolve nada disso. É um setup de uso local.

### 3.3 Migrations

```bash
go run . migrate        # ou: make migrate
```

`tracelog migrate` (spec 0003) cria a tabela `schema_migrations`, aplica em ordem os `migrations/*.sql` **embutidos no binário** que ainda não foram registrados e registra cada um logo depois. Um *advisory lock* impede duas execuções simultâneas de aplicar a mesma migration. Bancos migrados antes da existência do comando são adotados: tudo roda de novo, sem erro porque as migrations são idempotentes.

Saída real numa base vazia e, em seguida, numa segunda execução:

```text
[migrate] applied 000001_initial_schema
[migrate] applied 000002_add_log_import_fields
[migrate] applied 000003_external_id_per_application
[migrate] database is up to date (3 migrations)
[migrate] database is up to date (3 migrations)
```

| Arquivo | Conteúdo |
|---------|----------|
| `000001_initial_schema.sql` | Tabelas, FKs e índices (seção 8). Também cria a extensão `pgcrypto`, que nenhum código usa; foi mantida de propósito (spec 0003, seção 9). |
| `000002_add_log_import_fields.sql` | Colunas `raw_line`, `normalization_status`, `parse_error` e índice. |
| `000003_external_id_per_application.sql` | Troca o `UNIQUE (external_id)` global pelo índice único parcial `(application_id, external_id)`. |

### 3.4 Setup inicial

```bash
go run . init             # interativo; Enter aceita o padrão
go run . init --defaults  # sem perguntas (é o que o compose roda)
```

Grava uma aplicação (`Local Application`, chave `local-app`), um ambiente (`local`) e uma fonte (`HTTP API`, tipo `http`) e imprime os IDs. É um upsert por `monitored_application.service_key`, `environment.code` e `event_source(name, source_type)`: rodar de novo mantém os IDs. Para listá-los depois: `go run . db list applications|environments|sources`.

---

## 4. Compilar, executar e checar

```bash
go build -o bin/tracelog .   # ou: make build
make lint                    # gofmt (falha se listar algo) + go vet
make test                    # go test -race -count=1 ./...
make demo                    # demonstração de ponta a ponta (seção 5.4)
```

### 4.1 `serve`

```bash
go run . serve                    # porta do HTTP_PORT
go run . serve --http-port 8081   # sobrescreve a porta
```

Saída real (capturada num teste com banco em `127.0.0.1:55433` e `HTTP_PORT=18081`):

```text
[serve] tracelog (local)
[serve] Initializing PostgreSQL connection...
[serve] PostgreSQL connected at 127.0.0.1:55433/tracelog
[serve] Initializing application dependencies...
[serve] HTTP server listening on port 18081
[serve] Ready to receive events at POST /events
```

O `serve` escuta em `HTTP_HOST:HTTP_PORT` (padrão `127.0.0.1`, spec 0005); a mensagem acima mostra só a porta.

Sem o setup, falha com ``basic system setup is missing; run `tracelog init` before `tracelog serve` ``. `Ctrl+C` ou `SIGTERM` param de aceitar conexões, esperam as requisições em andamento por até 8s, fecham o pool e imprimem `[serve] Shutdown complete`.

Timeouts (spec 0002): cabeçalho 5s, leitura 15s, escrita 15s, ociosidade 60s, 10s por `POST /events` e por rota de leitura, 2s no `/ready`. Pool: até 10 conexões.

---

## 5. Uso

### 5.1 API HTTP

| Método | Rota | Resposta |
|--------|------|----------|
| `GET`, `HEAD` | `/health` | Liveness: `200 {"status":"ok"}`. Não consulta o banco. |
| `GET`, `HEAD` | `/ready` | Readiness: `200 {"status":"ready"}` ou `503 {"status":"unavailable"}`. |
| `POST` | `/events` | `201` se o evento é novo; `200` se já existia. |
| `GET`, `HEAD` | `/api/v1/...` | API de leitura (seção 5.1.1). |

Corpo de `POST /events` (todos os campos são opcionais):

| Campo | Tipo | Comportamento |
|-------|------|---------------|
| `message` | string | Vazio → `event received without message`. |
| `occurred_at` | string | RFC3339 (com ou sem fração/offset), `YYYY-MM-DDTHH:MM:SS` ou `YYYY-MM-DD HH:MM:SS` (sem fuso = UTC). Ausente ou inválido → hora de recebimento. Não é validado contra o futuro. |
| `category` | string | `application`, `system`, `security`, `audit` ou `unknown`. Ausente ou inválida → `unknown`. |
| `payload` | objeto | JSON livre, guardado como está (mais `_ingestion` quando há anotações). |
| `external_id` | string | Chave de deduplicação por aplicação. Vazio → sem deduplicação. Acima de 255 caracteres é truncado. |
| `trace_id`, `correlation_id` | string | Guardados e indexados. Acima de 128 caracteres são truncados. Não há filtro por eles na CLI. |
| `application_id`, `source_id`, `environment_id` | UUID | Qualquer versão. Ausente, inválido ou inexistente → o do setup do `init`, com anotação. |

- `severity` enviado pelo cliente é ignorado; `payload.level` é guardado, mas não define a severidade no HTTP.
- `Content-Type` pode ser omitido. Se enviado, precisa ser `application/json` (ou `*/*+json`). Atenção: `curl -d` sem `-H 'Content-Type: application/json'` manda `application/x-www-form-urlencoded` e recebe `415`.

Saída real da demo (seção 5.4): evento novo, reenvio do mesmo `external_id` e `Content-Type` errado:

```text
$ curl -X POST http://127.0.0.1:18083/events   # evento novo
{"id":"7a6206f0-bff1-4099-b85c-8746b5a0dee9","external_id":"demo-0001","received_at":"2026-11-20T20:58:24.671741Z","created_at":"2026-11-20T20:58:24.674644Z","duplicate":false}

[HTTP 201]

$ curl -X POST http://127.0.0.1:18083/events   # reenvio do mesmo external_id
{"id":"7a6206f0-bff1-4099-b85c-8746b5a0dee9","external_id":"demo-0001","received_at":"2026-11-20T20:58:24.671741Z","created_at":"2026-11-20T20:58:24.674644Z","duplicate":true}

[HTTP 200]

$ curl -X POST http://127.0.0.1:18083/events   # Content-Type que não é JSON
{"error":"Content-Type must be application/json"}

[HTTP 415]
```

| Erro | Quando |
|------|--------|
| `405` | Método não permitido; o cabeçalho `Allow` lista os aceitos. |
| `413` | Corpo maior que `HTTP_MAX_BODY_BYTES`. Nada é gravado. |
| `415` | `Content-Type` presente e diferente de JSON. Nada é gravado. |
| `500` | Falha interna (ex.: banco fora). Corpo `{"error":"internal error"}`; o detalhe vai para o log do servidor. |

JSON malformado **não** é erro: o evento é gravado com `payload._ingestion.decode_error` e `payload._ingestion.raw_request_body` (o corpo bruto só é guardado nesse caso).

#### 5.1.1 API de leitura (spec 0005)

Só `GET` e `HEAD`; outro método → `405` com `Allow: GET, HEAD`. Respostas com `Cache-Control: no-store` e `X-Content-Type-Options: nosniff`, horários em UTC e strings opcionais ausentes como `null`. Não há CORS: em outra origem, o navegador bloqueia a leitura.

| Rota | Parâmetros | Resposta |
|------|------------|----------|
| `/api/v1/events` | `severity`, `application_id`, `environment_id`, `from`, `to`, `limit` (1–200, padrão 50), `cursor` | `{"items":[...],"next_cursor":...}`; ordem `occurred_at` e `id` decrescentes; itens sem `payload`, `raw_line` e `parse_error` |
| `/api/v1/events/{id}` | — | Evento completo, com `payload` (inclusive `_ingestion`), `raw_line` e `parse_error` |
| `/api/v1/metrics/error-frequency` | `from` e `to` (obrigatórios, até 366 dias), `application_id`, `environment_id` | `error` e `fatal` por dia UTC, **com dias vazios preenchidos** |
| `/api/v1/metrics/severity-counts` | `application_id`, `environment_id`, `from`, `to` | As cinco severidades, sempre presentes, e `total` |
| `/api/v1/metrics/data-range` | — | Menor e maior `occurred_at`, último erro e `default_from`/`default_to`: 14 dias UTC até o último erro (ou o último evento, ou hoje) |
| `/api/v1/applications`, `/environments`, `/sources` | — | `{"items":[...]}`, sem paginação |

- `from`/`to`: RFC3339 ou `YYYY-MM-DD` (em `to`, o dia inteiro), como na CLI.
- Validação estrita, sem a tolerância da ingestão: parâmetro inválido, desconhecido (`severty=`) ou repetido → `400`. Valor vazio (`severity=`) conta como ausente. `application_id` válido mas inexistente → lista vazia.
- **Paginação por cursor:** repita a consulta com `cursor=<next_cursor>` e os mesmos filtros; `next_cursor: null` é a última página. O cursor é opaco (base64url de `occurred_at` + `id`), não é assinado e não é amarrado aos filtros. Eventos novos não deslocam as páginas; um evento importado com `occurred_at` antigo pode aparecer numa página ainda não visitada.

| Status | `code` | Quando |
|--------|--------|--------|
| `400` | `invalid_parameter` | Valor inválido ou fora da faixa, `from` depois de `to`, período acima de 366 dias, obrigatório ausente, parâmetro desconhecido ou repetido. `field` diz qual. |
| `400` | `invalid_cursor` | Cursor malformado. |
| `404` | `not_found` | Evento inexistente ou caminho desconhecido sob `/api/`. |
| `405` | `method_not_allowed` | Método diferente de `GET`/`HEAD`. |
| `500` | `internal` | Falha interna; o detalhe vai só para o log. |

Saída real, com `examples/logs/` importado e o evento `demo-0001` enviado (servidor em `127.0.0.1:18093`):

```text
$ curl -s 'http://127.0.0.1:18093/api/v1/events?severity=fatal&limit=1'
{"items":[{"id":"6979add7-40b6-4e11-9278-74851c22338c","external_id":"file:07ea4b6dc45822c1c7b1d43f239d99ca","occurred_at":"2026-03-10T14:47:00Z","received_at":"2026-11-23T21:39:54.843988Z","created_at":"2026-11-23T21:39:54.845594Z","severity":"fatal","category":"system","message":"order worker crashed: connection pool exhausted","normalization_status":"normalized","ingestion_channel":"file","trace_id":"5e669c5a5f102153","correlation_id":null,"application_id":"ff55e541-ea48-48a4-8f91-649a36bfd8b7","environment_id":"113c0027-4e23-4f9e-b191-540d051c5061","source_id":"f495f939-d088-4e42-a603-2fd08992ed20"}],"next_cursor":null}
[HTTP 200]

$ curl -s 'http://127.0.0.1:18093/api/v1/metrics/error-frequency?from=2026-03-01&to=2026-03-04'
{"from":"2026-03-01","to":"2026-03-04","days":[{"date":"2026-03-01","errors":2,"fatal":0,"total":2},{"date":"2026-03-02","errors":2,"fatal":0,"total":2},{"date":"2026-03-03","errors":1,"fatal":0,"total":1},{"date":"2026-03-04","errors":0,"fatal":0,"total":0}]}
[HTTP 200]

$ curl -s 'http://127.0.0.1:18093/api/v1/metrics/severity-counts?from=2026-03-01&to=2026-03-14'
{"counts":{"debug":0,"info":266,"warn":73,"error":39,"fatal":1},"total":379}
[HTTP 200]

$ curl -s 'http://127.0.0.1:18093/api/v1/metrics/data-range'
{"first_occurred_at":"2026-03-01T08:46:09Z","last_occurred_at":"2026-11-23T21:39:55.478686Z","last_error_at":"2026-03-14T18:00:00Z","default_from":"2026-03-01","default_to":"2026-03-14"}
[HTTP 200]

$ curl -s 'http://127.0.0.1:18093/api/v1/events?limit=201'
{"error":"limit must be an integer between 1 and 200","code":"invalid_parameter","field":"limit"}
[HTTP 400]

$ curl -s 'http://127.0.0.1:18093/api/v1/events?severty=error'
{"error":"unknown parameter \"severty\"","code":"invalid_parameter","field":"severty"}
[HTTP 400]

$ curl -s 'http://127.0.0.1:18093/api/v1/events?cursor=abc'
{"error":"cursor is malformed","code":"invalid_cursor","field":"cursor"}
[HTTP 400]

$ curl -s 'http://127.0.0.1:18093/api/v1/events/ffffffff-ffff-4fff-bfff-ffffffffffff'
{"error":"event not found","code":"not_found"}
[HTTP 404]
```

Respostas de referência de cada rota, usadas nos testes do handler e destinadas aos mocks do front: [`internal/adapters/http/testdata/api/`](../internal/adapters/http/testdata/api/). Para regravá-las depois de uma mudança consciente de contrato: `go test ./internal/adapters/http/ -run ReadAPIFixtures -update`.

**Desempenho medido** (fase 2 da trilha 2, `loggen -scale 200`, 80.266 eventos, cache quente, sem índice novo): primeira página e página profunda por cursor ~0,2 ms; `severity=error` 11–13 ms; frequência de erros 3–6 ms; contagem por severidade 31–43 ms (Seq Scan); via HTTP, de 1 a 51 ms por requisição. Detalhes e método no `PROGRESS.md`.

### 5.2 CLI

Com o binário, troque `go run .` por `tracelog`. No compose: `docker compose run --rm api <comando>`.

| Comando | Função | Flags |
|---------|--------|-------|
| `serve` | Sobe a API | `--http-port` |
| `migrate` | Aplica migrations pendentes | — |
| `init` | Grava o setup mínimo | `--defaults` |
| `import logs` | Importa arquivos de log | `--file` (obrigatório, repetível) |
| `events list` | Lista eventos, mais recentes (`occurred_at`) primeiro | `--severity`, `--application-id`, `--environment-id`, `--from`, `--to`, `--limit` (padrão 50) |
| `metrics errors` | `error` e `fatal` por dia (UTC) | `--application-id`, `--environment-id`, `--from`, `--to` |
| `db list environments\|applications\|sources` | Entidades do setup, com IDs | — |

`--from`/`--to` aceitam RFC3339 (usado como está) ou `YYYY-MM-DD`: em `--from` vale 00:00 UTC; em `--to`, o dia inteiro. Um UUID malformado em `--application-id`/`--environment-id` faz o PostgreSQL recusar a consulta e o comando falha com a mensagem dele.

Saída real da demo:

```text
$ tracelog events list --severity fatal --from 2026-03-01 --to 2026-03-14
OCCURRED_AT           SEVERITY  APPLICATION_ID                        ENVIRONMENT_ID                        MESSAGE
-----------           --------  --------------                        --------------                        -------
2026-03-10T14:47:00Z  fatal     192524eb-8a13-4ac4-afe1-42850aa716a5  6b6eae04-cf16-4999-9ec5-b805528e905b  order worker crashed: connection pool exhausted

Total: 1 event(s).
```

`metrics errors` está na seção 7.4.

### 5.3 Importação de arquivos de log

```bash
go run . import logs --file ./app.log --file ./worker.log
```

Cada linha não vazia vai para o primeiro parser que a reconhecer:

1. **JSON**: começa com `{`, ou com `[` e é JSON válido. Extrai `message`/`msg`/`error`/`event`/`description`, `timestamp`/`occurred_at`/`time`/`@timestamp`/`ts`, `level`/`severity`/`log_level`, `trace_id`, `correlation_id`/`request_id`, `category`. JSON quebrado → `partial` com `parse_error`.
2. **`timestamp nível mensagem`**: `2026-03-09T14:30:00Z ERROR ...` ou `[06/Mar/2026:02:26:01 +0000] WARN ...`. Timestamp ilegível → `partial` com `parse_error`.
3. **Texto livre**: o resto, `partial`.

Sem timestamp utilizável, `occurred_at` vira a hora da importação. Linhas em branco são puladas e contadas.

**Categoria** (palavras inteiras, na ordem): sinais de segurança (`unauthorized`, `forbidden`, `denied`, `token`, `credential`..., ou `login`/`auth` com `failed`/`invalid`/`locked`...) → `security`; `login`, `logout`, `permission`, `created by`... → `audit`; `database`, `timeout`, `connection`... → `system`; linha JSON ou `timestamp nível` sem palavra-chave → `application`; senão `unknown`. A categoria não muda o status: `normalized` só quando timestamp e mensagem foram lidos.

**Reimportação:** cada linha recebe `external_id = file:` + 32 caracteres hex de `sha256(nome do arquivo, número da linha, conteúdo)`. Reimportar o mesmo arquivo, de qualquer diretório, não grava nada novo. Mesmo nome, linha e conteúdo em arquivos diferentes contam como o mesmo evento (spec 0004).

**Desempenho:** a conexão do import usa `synchronous_commit=off`. Medido num i5-1135G7 com PostgreSQL 16 em Docker: 80.266 linhas em 34s (334s com commit síncrono). Se o PostgreSQL cair no meio, os commits dos últimos ~0,6s podem se perder, sem corromper nada; basta reimportar.

Saída real com `examples/logs/` num banco vazio:

```text
Log import summary
Files processed: 3
Lines read: 434
Persisted: 429
Duplicates (already imported): 0
Blank lines skipped: 5
Normalized: 378
Partially interpreted: 20
Uncategorizable: 31
```

Erros de gravação aparecem como `Persistence errors: N`, seguidos de até 10 linhas `arquivo:linha: motivo`.

### 5.4 Dados de exemplo e demonstração

- **`examples/logs/`**: 434 linhas sintéticas de 2026-03-01 a 2026-03-14 de uma aplicação fictícia, em três formatos (`checkout-api.log` JSON, `auth-service.log` `timestamp nível mensagem`, `legacy-gateway.log` texto livre). Pico de erros em 2026-03-10 (banco fora por 90 minutos, com um `fatal`), falhas de login em massa em 2026-03-06, linhas cortadas, sem timestamp e em branco. Usuários `user-NNNN` e IPs só das faixas de documentação (RFC 5737).
- **`tools/loggen`**: gerador desses arquivos, com semente fixa. `go run ./tools/loggen -out examples/logs` reproduz exatamente os arquivos versionados (há teste); `-scale N -out <dir>` gera volumes maiores, que não devem ser versionados.
- **`make demo`** (`scripts/demo.sh`): sobe o compose, importa os exemplos com a CLI dentro do container, envia eventos por `curl` e roda `events list` e `metrics errors`. Exige só Docker e `curl`. Saída real completa: [`demo-output.txt`](./demo-output.txt).
- **`docs/demo.tape`**: roteiro do [VHS](https://github.com/charmbracelet/vhs) para gravar `docs/demo.gif` com o stack no ar (`vhs docs/demo.tape`).

---

## 6. Normalização e fallback

- `occurred_at` ausente ou inválido → `received_at`; `message` vazia → texto padrão; `category` inválida → `unknown`.
- `application_id`, `source_id` ou `environment_id` ausente ou não-UUID → o do setup. Se o UUID não existir no banco, o `INSERT` falha por FK e o caso de uso tenta **uma** vez com o setup.
- O setup usado no fallback é sempre o registro **mais antigo** de cada tabela, mesmo que o `init` tenha sido rodado depois com outra chave.
- Toda troca é anotada em `payload._ingestion.normalization_warnings`.

---

## 7. Regras de negócio

### 7.1 Timestamps

`occurred_at` é o que o cliente informou (pode estar errado em máquinas de usuários); `received_at` é definido pelo servidor. Um nunca sobrescreve o outro, exceto no fallback acima.

### 7.2 Deduplicação

Unicidade em `(application_id, external_id)` onde `external_id` não é nulo (índice parcial). O `INSERT` usa `ON CONFLICT DO NOTHING` e, sem linha criada, devolve o evento existente; não há corrida entre duas requisições iguais.

### 7.3 Severidade

A primeira regra que casar vence. Palavras da mensagem são comparadas inteiras, sem diferenciar maiúsculas; a lista completa, com flexões, fica em `register_technical_event_usecase.go`.

1. **Só fora do HTTP** (import): `payload.level` reconhecido (`trace`/`debug`, `info`, `warn`/`warning`, `error`/`err`, `fatal`/`critical`/`panic`).
2. `panic`, `fatal`, `unrecoverable`, `crash` (e flexões) na mensagem, ou `panic`/`fatal`/`stack_trace` no payload → `fatal`.
3. `error`, `failed`, `failure`, `timeout`, `exception`, `unauthorized`, `denied` (e flexões), um tipo CamelCase terminado em `Error`/`Exception`, ou `error`/`errors`/`exception`/`exception_message` no payload → `error`.
4. Categoria `security`, `warn`, `warning`, `retry`, `deprecated`, `slow`, `fallback` (e flexões), ou `warning`/`warnings`/`retry_count` no payload → `warn`.
5. Senão → `info`.

**Limitação:** é uma heurística sem noção de negação ou contexto. `0 errors found`, `report generated without failure` e `retry succeeded after timeout` viram `error` (documentado em `severity_test.go`).

### 7.4 `metrics errors`

Conta `error` e `fatal` em colunas separadas, por dia em UTC, independentemente do fuso da sessão do PostgreSQL. Dias sem `error` nem `fatal` não aparecem (preencher dias vazios é da camada de apresentação). Saída real da demo:

```text
$ tracelog metrics errors --from 2026-03-01 --to 2026-03-14
DATE        ERRORS  FATAL  TOTAL
----        ------  -----  -----
2026-03-01  2       0      2
2026-03-02  2       0      2
2026-03-03  1       0      1
2026-03-05  1       0      1
2026-03-06  1       0      1
2026-03-08  1       0      1
2026-03-09  2       0      2
2026-03-10  25      1      26
2026-03-11  1       0      1
2026-03-13  1       0      1
2026-03-14  2       0      2

Summary:
  Days with errors: 11
  Errors: 39
  Fatal: 1
  Average (error+fatal) per day with errors: 3.6
```

---

## 8. Modelo de dados

Tabelas (nomes no singular), de `migrations/`:

| Tabela | Colunas principais | Restrições |
|--------|--------------------|------------|
| `environment` | `id`, `code`, `name`, `description`, `created_at` | `code` único |
| `monitored_application` | `id`, `name`, `service_key`, `description`, `created_at` | `service_key` único |
| `event_source` | `id`, `name`, `source_type`, `description`, `created_at` | `(name, source_type)` único |
| `technical_event` | `id`, `external_id`, `occurred_at`, `received_at`, `message`, `severity`, `category`, `payload` (JSONB), `trace_id`, `correlation_id`, `raw_line`, `normalization_status`, `parse_error`, `ingestion_channel`, `application_id`, `source_id`, `environment_id`, `created_at` | FKs para as três tabelas acima; único parcial `(application_id, external_id)` |
| `schema_migrations` | `version`, `applied_at` | criada pelo `tracelog migrate` |

Índices de `technical_event`: `occurred_at DESC`, `severity`, `application_id`, `environment_id`, `trace_id`, `correlation_id`, `normalization_status` e o único parcial de deduplicação. As consultas da API de leitura foram medidas com esses índices na trilha 2 (seção 5.1.1); nenhum índice novo foi criado.

Valores: `severity` ∈ `debug`, `info`, `warn`, `error`, `fatal`; `category` ∈ `application`, `system`, `security`, `audit`, `unknown`; `ingestion_channel` ∈ `http`, `file` (`cli` existe no domínio, mas nenhum comando o usa); `normalization_status` ∈ `normalized`, `partial`, `uncategorizable`.

---

## 9. Arquitetura

```text
tracelog/
├── main.go                     # chama cmd.Execute()
├── cmd/                        # CLI (Cobra): serve, migrate, init, import, events, metrics, db
├── internal/
│   ├── config/                 # .env + ambiente, validação, DSN
│   ├── core/
│   │   ├── domain/             # entidades e enums, sem dependências externas
│   │   ├── ports/              # interfaces dos repositórios
│   │   └── services/           # casos de uso (testes com fakes)
│   └── adapters/
│       ├── http/               # servidor, ingestão, API de leitura, DTOs, testdata/api
│       └── repository/         # PostgreSQL (pgx/v5 via database/sql) e migrator
├── migrations/                 # *.sql + embed.go
├── tools/loggen/               # gerador do dataset
├── examples/logs/              # dataset versionado
├── scripts/demo.sh             # demonstração
├── Dockerfile, docker-compose.yml, Makefile
└── .github/workflows/ci.yml
```

Dependências apontam para dentro: `cmd` e `adapters/http` chamam `services`, que só conhecem `domain` e `ports`; `adapters/repository` implementa os `ports`. A leitura usa `TechnicalEventRepository` (com `FindByID` e cursor), `ErrorFrequencyRepository`, `EventStatsRepository` e `ReferenceDataRepository`; erros de entrada são `services.ValidationError` e evento inexistente é `ports.ErrNotFound`, que o adapter HTTP traduz em `400`/`404`. Dependências diretas: `github.com/jackc/pgx/v5` e `github.com/spf13/cobra`.

**Desvios conscientes da arquitetura:**

- `cmd/migrate.go` chama `repository.Migrate` sem caso de uso: migração é infraestrutura, não regra de negócio (spec 0003).
- Parte da saída de `init`, `serve`, `events list` e `db list` vai para stderr (`cmd.Println` do Cobra); a tabela vai para stdout. `metrics errors` e `import logs` escrevem tudo em stdout.

---

## 10. Testes

```bash
go test ./...                                    # sem banco: testes de integração ficam em SKIP
TRACELOG_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/tracelog?sslmode=disable' \
  go test -race ./...                            # com integração
```

| Pacote | Funções `Test*` | Cobre |
|--------|-----------------|-------|
| `internal/core/services` | 37 | Tabelas de severidade, categoria e parser; normalização, truncagem, fallback e retry; duplicata; import (linhas vazias, erros, reimportação); `init`; listagem; leitura (paginação com timestamps iguais, última página cheia, validação, dias preenchidos em UTC, período padrão) |
| `internal/adapters/http` | 23 | Ingestão com `httptest` (201, 200, 405, 413, 415, JSON inválido, contexto, erro sem vazamento, fallback), `/health`, `/ready`, shutdown; leitura contra as fixtures de `testdata/api`, parâmetros inválidos, cursor adulterado, escape de HTML, cabeçalhos, 404/405 |
| `internal/adapters/repository` | 16 | 12 de integração com PostgreSQL (deduplicação, FK, filtros, `metrics` em UTC, migrate, adoção de banco legado, idempotência do setup, paginação com timestamps iguais, busca por ID, agregados, referência), pool, DSN do import, erro de FK |
| `internal/config` | 6 | Defaults, obrigatórias, `.env`, `HTTP_MAX_BODY_BYTES`, `HTTP_HOST`, escape do DSN |
| `cmd` | 5 | Saídas de `metrics` e `import`, `--from`/`--to`, `init --defaults` |
| `tools/loggen` | 2 | Determinismo e igualdade com `examples/logs` |

Os testes de integração criam e apagam um schema próprio por teste, sem tocar nas tabelas existentes. O CI (`.github/workflows/ci.yml`) roda gofmt, `go vet`, `go test -race` contra um PostgreSQL 16 de serviço, `go build` e, num segundo job, `docker build` com checagem de que a imagem não roda como root.

Lacuna: não há teste automatizado de ponta a ponta da CLI contra banco; a demo cumpre esse papel manualmente.

---

## 11. Como estender

- **Caso de uso:** `internal/core/services/` + teste com fake do repositório.
- **Persistência:** declarar em `internal/core/ports/`, implementar em `internal/adapters/repository/`, cobrir com teste de integração.
- **Rota HTTP:** `internal/adapters/http/server.go`, com teste `httptest`.
- **Comando:** arquivo em `cmd/` com `rootCmd.AddCommand(...)`; escrever em `cmd.OutOrStdout()`.
- **Migration:** `migrations/NNNNNN_descricao.sql`, idempotente e com `BEGIN`/`COMMIT`. Nunca edite uma já commitada.
- **Mudança de contrato** (rota, resposta, comando, schema, configuração): spec em `docs/specs/` antes do código.

---

## 12. Limitações conhecidas

1. Sem autenticação, TLS e rate limit (spec 0003, seção 5).
2. Heurística de severidade sem negação (seção 7.3).
3. Truncagem de `external_id`: valores longos com os mesmos 255 primeiros caracteres viram o mesmo evento (spec 0002).
4. Import: mesmo nome de arquivo, linha e conteúdo em arquivos diferentes viram o mesmo evento (spec 0004); com `synchronous_commit=off`, uma queda do PostgreSQL pode perder os últimos ~0,6s de commits.
5. Fallback sempre usa o setup mais antigo (seção 6).
6. `--help` exige as variáveis do banco; o `.env` não aceita comentário no fim da linha.
7. `events list` corta a mensagem em 100 bytes e pode quebrar um caractere UTF-8 no meio; `--limit` não tem teto.
8. Migrations não têm *down*: correção é sempre uma migration nova.
9. A API de leitura devolve tudo o que foi gravado, sem redação: dado sensível enviado por um cliente é lido por quem alcançar a porta. O bind em `127.0.0.1` é mitigação, não controle de acesso.

---

## 13. Próximos passos

Na ordem do roadmap do `AGENTS.md`:

1. Núcleo de investigação: entrada por pipe, busca, agrupamento por fingerprint, visão por `trace_id`.
2. SQLite como padrão, com PostgreSQL opcional (exige reescrever a camada de repositório).
3. API JSON de leitura e UI web embutida.
4. MCP e busca em linguagem natural.
5. Dashboards.

Pendências do MVP: lote e `external_id` obrigatório (spec 0001, D3/D5); saída da CLI toda em stdout; teste de ponta a ponta da CLI.
