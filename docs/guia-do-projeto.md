# Guia do Projeto TraceLog

Documento de referência para retomar o desenvolvimento do TraceLog. Descreve o estado atual do código, como instalar, compilar, executar e testar, e o que ainda precisa de atenção.

> Este guia foi escrito a partir do código no commit `456c830` e atualizado com as mudanças da spec 0002 (fase 2, 2026-11-20). Quando algo aqui divergir do código, o código prevalece. As divergências conhecidas com os outros documentos de `docs/` estão listadas na seção [Divergências conhecidas](#divergências-conhecidas-na-documentação).

---

## 1. Visão geral

TraceLog é um MVP local de monitoramento de eventos técnicos, escrito em Go com PostgreSQL. Ele:

- recebe eventos de aplicações via HTTP (`POST /events`);
- importa eventos a partir de arquivos de log (`tracelog import logs`);
- persiste tudo em PostgreSQL;
- oferece uma CLI para consultar eventos, métricas de erro e entidades de referência.

Fora do escopo atual: dashboards, consumidores em background, alertas, provisionamento automático de ambientes e RabbitMQ (previsto em `docs/architecture.md`, ainda não implementado).

---

## 2. O que já foi feito

Histórico resumido. Os commits completos estão em `git log`.

| Área | Entrega | Commits de referência |
|------|---------|----------------------|
| HTTP | Servidor com `POST /events` e `GET /health`, respostas JSON | `d4cc38e`, `2d01da8` |
| HTTP (spec 0002) | Timeouts, graceful shutdown, limite de corpo (413), `415`, erros sem vazamento, deduplicação por aplicação, `/ready` | branch `fix/http-hardening` |
| Configuração | Carga de `.env` na raiz e variáveis de ambiente, com validação | `ddf3c6b`, `6aa3009`, `2eece1b` |
| CLI | Estrutura com Cobra; comandos `serve`, `events list`, `metrics errors`, `db list` | `812cccc`, `e9f51b8`, `c8047b1` |
| Domínio | Entidades `TechnicalEvent`, `MonitoredApplication`, `EventSource`, `Environment`; enums de severidade, categoria e canal | `eaa0014`, `ce03421` |
| Casos de uso | Registro de evento, listagem com filtros, frequência de erros | `1e92d7f`, `d0d78be`, `c4bba8b` |
| Setup inicial | `tracelog init` (interativo) e verificação obrigatória no `serve` | `812cccc`, `269d429`, `979ee17`, `3f151f9` |
| Banco | Migrations SQL (schema inicial e campos de importação) | `9d171f1`, `7c12bf6` |
| Severidade | Derivada no servidor a partir de nível, mensagem e payload (não confiada ao cliente) | `4467ad9`, `767c699` |
| Normalização | Ingestão tolerante: campos ausentes/inválidos são normalizados e anotados em `payload._ingestion` | `3e347cf` |
| Fallback de IDs | Se `application_id`, `source_id` ou `environment_id` faltarem/forem inválidos, usa-se o setup de `init`; há nova tentativa após erro de FK | `3e347cf` |
| Importação de logs | `tracelog import logs --file ...` com parsers para JSON, `timestamp level message` e texto puro | `3e347cf`, `a700da1` |
| Testes | Testes unitários de casos de uso, config e mapeamento de erro de FK | `699cfd0` e posteriores |
| Documentação | `README.md` e `docs/` (getting-started, cli, http-api, domain, architecture, development) | `55d2662`, `a3f9261`, `147ea40`, `72aa48e` |

### Decisões de comportamento que importam

- **`severity` não é aceito do cliente no HTTP.** Ela é derivada pelo servidor (ver seção 7.3).
- **Ingestão tolerante.** O HTTP só responde erro quando não consegue persistir nada. Dados incompletos são gravados com anotações.
- **`serve` exige setup.** Se `tracelog init` não foi executado, o servidor não sobe.
- **`seed` é um no-op.** Ele apenas imprime uma mensagem. Os dados de referência vêm do `init`.

---

## 3. Pré-requisitos

| Ferramenta | Versão | Observação |
|------------|--------|------------|
| Go | **1.26.11** (conforme `go.mod`) | Os docs antigos citam 1.21 e 1.25; a referência válida é o `go.mod`. Com `GOTOOLCHAIN=auto` (padrão), um Go mais antigo baixa essa versão sozinho. |
| Docker + Docker Compose | Plugin `docker compose` | Usado para subir o PostgreSQL 16. |
| `psql` | Opcional | Só para inspeção manual; os scripts usam `psql` dentro do container. |
| Git | Qualquer versão recente | |

---

## 4. Instalação e configuração

Execute tudo a partir da **raiz do repositório**. Isso é obrigatório: o `.env` é lido do diretório atual (`config.go` usa o caminho relativo `.env`).

### 4.1 Clonar

```bash
git clone <url-do-repositorio> tracelog
cd tracelog
```

### 4.2 Criar o `.env`

```bash
cp .env.example .env
```

Conteúdo padrão (`.env.example`):

```env
APP_NAME=tracelog
APP_ENV=local
HTTP_PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_NAME=tracelog
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSLMODE=disable
```

| Variável | Obrigatória | Padrão | Descrição |
|----------|-------------|--------|-----------|
| `APP_NAME` | não | `tracelog` | Nome exibido no log do `serve`. |
| `APP_ENV` | não | `local` | Ambiente exibido no log do `serve`. |
| `HTTP_PORT` | não | `8080` | Porta do servidor HTTP (1–65535). |
| `HTTP_MAX_BODY_BYTES` | não | `1048576` | Tamanho máximo do corpo de `POST /events`, em bytes. Acima disso: `413`. Precisa ser positivo. |
| `DB_HOST` | não | `localhost` | Host do PostgreSQL. |
| `DB_PORT` | não | `5432` | Porta do PostgreSQL; também é a porta publicada pelo Docker. |
| `DB_NAME` | **sim** | — | Nome do banco. |
| `DB_USER` | **sim** | — | Usuário do banco. |
| `DB_PASSWORD` | **sim** | — | Senha do banco. |
| `DB_SSLMODE` | não | `disable` | Um de `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`. |

**Precedência:** variável de ambiente do shell > `.env`. Um `.env` nunca sobrescreve uma variável já definida.

> **Atenção:** sem `DB_NAME`, `DB_USER` e `DB_PASSWORD`, **qualquer** comando do binário falha antes de executar, inclusive `--help`.

### 4.3 Subir com Docker Compose

Caminho mais curto: tudo em container.

```bash
docker compose up -d     # ou: make up
docker compose ps -a
```

O `docker-compose.yml` (spec 0003) tem quatro serviços:

| Serviço | Faz | Começa quando |
|---------|-----|---------------|
| `postgres` | `postgres:16.16-alpine`, volume `tracelog_postgres_data`, healthcheck `pg_isready` | — |
| `migrate` | `tracelog migrate` e termina | `postgres` saudável |
| `init` | `tracelog init --defaults` e termina | `migrate` terminou com código 0 |
| `api` | `tracelog serve`, healthcheck em `/ready` | `init` terminou com código 0 |

`migrate` e `init` aparecem como `exited (0)` no `ps -a`: é o esperado. Rodar `docker compose up` de novo não reaplica migrations nem muda IDs.

- Dentro da rede do compose a aplicação usa `DB_HOST=postgres` e `DB_PORT=5432`. O `DB_HOST`/`DB_PORT` do `.env` só valem para a CLI no host.
- PostgreSQL e API são publicados **só em `127.0.0.1`**, nas portas `DB_PORT` e `HTTP_PORT` do `.env`. Se a 5432 já estiver ocupada por outro PostgreSQL, troque `DB_PORT` no `.env`.
- A imagem `tracelog:local` é construída do `Dockerfile` (multi-stage, `alpine:3.24`, processo com UID 10001, sem root).

Só o banco, para usar a CLI no host:

```bash
docker compose up -d postgres
```

**O que não é seguro neste setup:** a API não tem autenticação (quem alcança a porta grava eventos), não há TLS (nem cliente→API nem API→banco), a senha padrão do banco é `postgres` e não há rate limit. Publicar só em `127.0.0.1` reduz a exposição, mas não resolve nada disso. É um setup de uso local.

Para zerar o banco (apaga os dados):

```bash
docker compose down -v
```

### 4.4 Aplicar as migrations

```bash
go run . migrate        # ou: make migrate
```

O comando (spec 0003):

1. usa o banco configurado no `.env`/ambiente (o PostgreSQL precisa estar no ar);
2. cria a tabela `schema_migrations` se ela não existir;
3. aplica, em ordem, os `migrations/*.sql` **embutidos no binário** que ainda não estão registrados, um por vez, registrando cada um logo depois;
4. usa um *advisory lock* para que duas execuções simultâneas não apliquem a mesma migration.

Saída real numa base vazia e numa segunda execução:

```text
[migrate] applied 000001_initial_schema
[migrate] applied 000002_add_log_import_fields
[migrate] applied 000003_external_id_per_application
[migrate] database is up to date (3 migrations)
[migrate] database is up to date (3 migrations)
```

O antigo `scripts/run-migrations.sh` foi removido. Um banco migrado por ele (sem `schema_migrations`) é adotado: tudo roda de novo, sem erro porque os arquivos são idempotentes, e passa a ser registrado.

Migrations atuais:

| Arquivo | Conteúdo |
|---------|----------|
| `000001_initial_schema.sql` | Tabelas `environment`, `monitored_application`, `event_source`, `technical_event`, FKs e índices. |
| `000002_add_log_import_fields.sql` | Colunas `raw_line`, `normalization_status`, `parse_error` em `technical_event` e índice. |
| `000003_external_id_per_application.sql` | Troca o `UNIQUE (external_id)` global por índice único parcial em `(application_id, external_id)` onde `external_id` não é nulo. |

> **Importante:** toda migration nova continua tendo que ser idempotente (`IF NOT EXISTS`): se o processo morrer entre aplicar um arquivo e registrá-lo, ele roda de novo na próxima vez.

### 4.5 Setup inicial (obrigatório antes do `serve`)

```bash
go run . init
```

É interativo: pressione Enter para aceitar os padrões. Para não perguntar nada (containers, scripts), use `go run . init --defaults`, que grava os mesmos padrões sem ler a entrada; é isso que o serviço `init` do compose roda.

| Pergunta | Padrão |
|----------|--------|
| Application name | `Local Application` |
| Application service key | `local-app` |
| Environment code | `local` |
| Environment name | `Local` |
| Event source name | `HTTP API` |
| Event source type | `http` |

Ao final, a saída mostra os IDs gravados (application, environment, source). **Guarde esses IDs**: são usados nas chamadas HTTP e nos filtros da CLI. Para listá-los depois:

```bash
go run . db list applications
go run . db list environments
go run . db list sources
```

O `init` é um upsert: rodar de novo atualiza os registros existentes (chaves: `environment.code`, `monitored_application.service_key`, `event_source(name, source_type)`).

---

## 5. Compilar e executar

### 5.1 Build do binário

```bash
go build -o bin/tracelog .
./bin/tracelog --help
```

`bin/` está no `.gitignore`. Lembre que o `--help` só funciona com o `.env` presente no diretório atual (seção 4.2).

### 5.2 Rodar sem gerar binário

```bash
go run . <comando> [flags]
```

Exemplos: `go run . serve`, `go run . events list --limit 10`.

### 5.3 Checagens de qualidade

```bash
gofmt -l .          # lista arquivos fora do padrão (vazio = ok)
go vet ./...
go test ./...
```

O `Makefile` tem atalhos: `make lint` (gofmt + vet), `make test` (com `-race`), `make build`, `make up` e `make migrate`.

### 5.4 Subir o servidor

```bash
go run . serve                  # usa HTTP_PORT do .env
go run . serve --http-port 8081 # sobrescreve a porta
```

Saída esperada:

```text
[serve] tracelog (local)
[serve] Initializing PostgreSQL connection...
[serve] PostgreSQL connected at localhost:5432/tracelog
[serve] Initializing application dependencies...
[serve] HTTP server listening on port 8080
[serve] Ready to receive events at POST /events
```

Se o setup estiver ausente, o `serve` falha com: `basic system setup is missing; run \`tracelog init\` before \`tracelog serve\``.

Para encerrar: `Ctrl+C` (SIGINT) ou `SIGTERM`. O servidor para de aceitar conexões, espera as requisições em andamento por até 8s, fecha o pool do banco e imprime `[serve] Shutdown complete`.

Timeouts do servidor (justificados na spec 0002): leitura de cabeçalho 5s, leitura 15s, escrita 15s, ociosidade 60s, 10s por requisição de `POST /events`. Pool do banco: até 10 conexões.

---

## 6. Uso

### 6.1 API HTTP

Base: `http://localhost:8080` (ou a porta configurada).

| Método | Rota | Função |
|--------|------|--------|
| `GET`, `HEAD` | `/health` | Liveness: `200 {"status":"ok"}`. Não consulta o banco. |
| `GET`, `HEAD` | `/ready` | Readiness: ping no banco com timeout de 2s. `200 {"status":"ready"}` ou `503 {"status":"unavailable"}`. |
| `POST` | `/events` | Registra um evento. `201` se novo, `200` se já existia (ver deduplicação). |

Corpo do `POST /events`:

| Campo | Tipo | Obrigatório | Comportamento real |
|-------|------|-------------|--------------------|
| `message` | string | não | Se vazio, grava `event received without message`. |
| `occurred_at` | string | não | RFC3339 (com ou sem fração/offset) ou `YYYY-MM-DD HH:MM:SS`. Ausente/inválido → usa o horário de recebimento. |
| `category` | string | não | `application`, `system`, `security`, `audit`, `unknown`. Ausente/inválido → `unknown`. |
| `payload` | objeto | não | JSON livre. |
| `external_id` | string | não | Vazio vira `NULL` (sem deduplicação). Acima de 255 caracteres é truncado. |
| `trace_id` | string | não | Acima de 128 caracteres é truncado. |
| `correlation_id` | string | não | Acima de 128 caracteres é truncado. |
| `application_id` | UUID | não* | Qualquer versão de UUID. Ausente/inválido → setup do `init`. |
| `source_id` | UUID | não* | Ausente/inválido → setup do `init`. |
| `environment_id` | UUID | não* | Ausente/inválido → setup do `init`. |

\* Na prática são opcionais, desde que o `init` tenha sido executado (há fallback). Use sempre os IDs retornados pelo `init` para não depender do fallback.

**Campos que não existem mais no contrato:** `severity` é **ignorado** se enviado, e `payload.level` é guardado mas não define a severidade. O servidor deriva a severidade (seção 7.3).

`Content-Type` pode ser omitido; se enviado, precisa ser `application/json` (ou `*/*+json`), senão a resposta é `415`.

**Deduplicação:** dois eventos com o mesmo `external_id` na mesma aplicação (`application_id` já normalizado) são o mesmo evento. O reenvio responde `200` com os dados do evento já gravado e `"duplicate": true`. Aplicações diferentes podem repetir o mesmo `external_id`.

Exemplo:

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Database connection timeout",
    "occurred_at": "2026-03-09T14:30:00Z",
    "category": "system",
    "application_id": "<application-id-do-init>",
    "source_id": "<source-id-do-init>",
    "environment_id": "<environment-id-do-init>",
    "payload": {"retry_count": 3, "timeout_ms": 5000}
  }'
```

Respostas reais capturadas no teste de fumaça da fase 2 (evento novo e reenvio com o mesmo `external_id`):

```text
{"id":"432580d7-1242-4270-99d8-7198fef69d24","external_id":"smoke-1","received_at":"2026-11-20T16:03:43.446455-03:00","created_at":"2026-11-20T16:03:43.449417-03:00","duplicate":false}
  [HTTP 201]
{"id":"432580d7-1242-4270-99d8-7198fef69d24","external_id":"smoke-1","received_at":"2026-11-20T16:03:43.446455-03:00","created_at":"2026-11-20T16:03:43.449417-03:00","duplicate":true}
  [HTTP 200]
```

Erros:

| Status | Quando |
|--------|--------|
| `405` | Método não permitido; o cabeçalho `Allow` lista os aceitos. |
| `413` | Corpo maior que `HTTP_MAX_BODY_BYTES`. Nada é gravado. |
| `415` | `Content-Type` presente e diferente de JSON. Nada é gravado. |
| `500` | Falha interna (ex.: banco fora do ar). A resposta é só `{"error":"internal error"}`; o detalhe vai para o log do servidor. |

JSON malformado **não** gera erro: o evento é gravado com `payload._ingestion.decode_error` e `payload._ingestion.raw_request_body`. O corpo bruto só é guardado nesse caso.

### 6.2 CLI

Com o binário compilado, substitua `go run .` por `tracelog`.

| Comando | Função | Flags principais |
|---------|--------|------------------|
| `serve` | Sobe o HTTP | `--http-port` |
| `init` | Setup inicial interativo | — |
| `events list` | Lista eventos (mais recentes primeiro) | `--severity`, `--application-id`, `--environment-id`, `--from`, `--to`, `--limit` (padrão 50) |
| `metrics errors` | Contagem de `error` e `fatal` por dia (UTC) | `--application-id`, `--environment-id`, `--from`, `--to` |
| `db list environments\|applications\|sources` | Lista entidades de referência com IDs | — |
| `import logs` | Importa arquivos de log | `--file` (obrigatório, repetível) |
| `seed` | **No-op** (apenas imprime mensagem) | — |

Formatos de data em `--from`/`--to`: RFC3339 (`2026-03-09T14:30:00Z`, usado como está) ou `2026-03-09`. Só a data em `--from` vale 00:00 UTC; em `--to` vale o dia inteiro (até 23:59:59.999999999 UTC).

Exemplos:

```bash
go run . events list --severity error --limit 20
go run . events list --application-id <uuid> --from 2026-03-01 --to 2026-03-09
go run . metrics errors --environment-id <uuid> --from 2026-03-01
go run . db list applications
```

### 6.3 Importação de arquivos de log

```bash
go run . import logs --file ./app.log
go run . import logs --file ./app.log --file ./worker.log
```

Cada linha é tratada por um parser, na ordem:

1. **JSON** (linha começa com `{` ou `[`). Extrai `message`/`msg`/`error`, `trace_id`, `correlation_id`/`request_id`, `level`, `timestamp`/`ts`, etc. JSON inválido → `partial` com `parse_error`.
2. **`timestamp level message`** (ex.: `2026-03-09T14:30:00Z ERROR falha de conexão`). Timestamp inválido → `partial`.
3. **Texto puro** (fallback). Linha vazia → `uncategorizable`.

A categoria é inferida por palavras-chave na mensagem (ex.: `login`/`auth` → `audit`; `denied`/`token` → `security`; `database`/`timeout`/`connection` → `system`). Se nada casar, vira `unknown`.

Saída:

```text
Log import summary
Files processed: 2
Lines read: 482
Persisted: 482
Normalized: 301
Partially interpreted: 143
Uncategorizable: 38
```

Cada evento importado recebe `ingestion_channel=file`, `raw_line`, `normalization_status`, `parse_error` e `payload._ingestion` (arquivo, linha, formato, parser).

---

## 7. Regras de negócio

### 7.1 Setup e fallback

- `init` grava application, environment e source. `serve` e `import` falham se o setup não existir.
- Quando um ID de referência está ausente ou não é UUID, o caso de uso usa o setup do banco e anota o fato em `payload._ingestion.normalization_warnings`.
- Se o INSERT falhar por FK inexistente (`application_id`, `source_id` ou `environment_id`), o caso de uso recarrega o setup e tenta **uma** vez novamente.

### 7.2 Normalização

Aplicada em `register_technical_event_usecase.go`:

- `occurred_at` ausente ou inválido → `received_at`;
- `message` vazia → texto padrão;
- `category` inválida → `unknown`;
- `normalization_status` inválido → `partial`;
- `ingestion_channel` deve ser `http`, `cli` ou `file`.

### 7.3 Derivação de severidade

Ordem de avaliação (a primeira regra que casar vence). As palavras da mensagem são comparadas como **palavras inteiras**, sem diferenciar maiúsculas; a lista completa, com flexões (`crashed`, `timeouts`...), fica em `register_technical_event_usecase.go`.

1. **Só fora do HTTP** (ex.: `import logs`): `payload.level` com valor reconhecido (`trace`/`debug`, `info`, `warn`/`warning`, `error`/`err`, `fatal`/`critical`/`panic`). No HTTP esse campo é ignorado para a severidade.
2. Mensagem tem `panic`, `fatal`, `unrecoverable`, `crash` (e flexões), **ou** payload tem `panic`, `fatal` ou `stack_trace` com valor significativo → `fatal`.
3. Mensagem tem `error`, `failed`, `failure`, `timeout`, `exception`, `unauthorized`, `denied` (e flexões), **ou** um nome de tipo CamelCase terminado em `Error`/`Exception` (`TypeError`, `IOException`), **ou** payload tem `error`, `errors`, `exception` ou `exception_message` → `error`.
4. Categoria `security`, **ou** mensagem tem `warn`, `warning`, `retry`, `deprecated`, `slow`, `fallback` (e flexões), **ou** payload tem `warning`, `warnings` ou `retry_count` → `warn`.
5. Caso contrário → `info`.

Limitações conhecidas (documentadas em `severity_test.go`): a heurística não entende negação nem contexto. `0 errors found`, `report generated without failure` e `retry succeeded after timeout` continuam `error`.

### 7.4 Métricas

`metrics errors` conta `severity = 'error'` e `severity = 'fatal'` em colunas separadas (`ERRORS`, `FATAL`, `TOTAL`), agrupando por dia em UTC, independentemente do fuso do PostgreSQL. Dias sem `error` nem `fatal` não aparecem. Tabela e resumo vão para a saída padrão.

---

## 8. Testes

### 8.1 Rodar

```bash
go test ./...                                   # tudo
go test -v ./internal/core/services/...         # um pacote, verboso
go test -v -run TestImportLogFiles ./internal/core/services/   # um teste
go test -coverprofile=coverage.out ./...        # cobertura
go tool cover -html=coverage.out
```

Estado na fase 2 (2026-11-20): `go vet ./...` limpo e `go test -race ./...` passando, inclusive os testes de integração com PostgreSQL 16.

### 8.2 Cobertura atual dos testes

| Pacote | Testes | O que cobre |
|--------|--------|-------------|
Contagem de funções `Test*` de nível superior (subtestes de tabela não somados):

| Pacote | Testes | O que cobre |
|--------|--------|-------------|
| `internal/core/services` | 19 | Tabela da heurística de severidade, `payload.level` por canal, fallback de IDs e retry só para `ErrReferenceNotFound`, truncagem, corpo bruto, UUID, duplicata, importação, `init`, listagem. |
| `internal/adapters/http` | 15 | Handler com `httptest` (201, 200 duplicata, JSON inválido gravado, 405, 413, 415, contexto, erro sem vazamento, fallback de FK), `/health`, `/ready`, graceful shutdown. |
| `internal/adapters/repository` | 7 | 4 de integração com PostgreSQL (deduplicação, FK, filtros, `metrics errors` em UTC) + pool e mapeamento de erro de FK. |
| `internal/config` | 4 | Defaults, obrigatórias, `.env`, validação de `HTTP_MAX_BODY_BYTES`. |
| `cmd` | 2 | Saída de `metrics errors` e limites de `--from`/`--to`. |

Os testes de `services` e `http` usam fakes em memória. Os de integração só rodam com `TRACELOG_TEST_DATABASE_URL` (ex.: `postgres://postgres:postgres@127.0.0.1:5432/tracelog?sslmode=disable`); cada teste cria e apaga um schema próprio, sem tocar nas tabelas existentes. Sem a variável eles aparecem como `SKIP`. O CI roda com um PostgreSQL 16 de serviço.

### 8.3 Teste manual de ponta a ponta

Com o PostgreSQL no ar, migrations aplicadas e `init` executado:

```bash
# terminal 1
go run . serve

# terminal 2
curl -s http://localhost:8080/health
curl -s -X POST http://localhost:8080/events -H "Content-Type: application/json" \
  -d '{"message":"Teste manual: timeout no banco","occurred_at":"2026-03-09T14:30:00Z","category":"system","application_id":"<app>","source_id":"<src>","environment_id":"<env>"}'
go run . events list --limit 5
go run . metrics errors
```

Para importar um arquivo de teste:

```bash
printf '%s\n' '2026-03-09T14:30:00Z ERROR connection timeout to postgres' '{"level":"warn","message":"retry 2"}' 'linha qualquer' > /tmp/tracelog-sample.log
go run . import logs --file /tmp/tracelog-sample.log
```

### 8.4 Lacunas de teste

- O `docs/development.md` descreve um banco `tracelog_test`; os testes de integração reais usam `TRACELOG_TEST_DATABASE_URL` (seção 8.2).
- Os comandos da CLI só têm testes das funções de saída e de parsing, não da execução contra banco.

---

## 9. Estrutura do projeto

```text
tracelog/
├── main.go                       # entrada: chama cmd.Execute()
├── cmd/                          # adapters de CLI (Cobra)
│   ├── root.go                   # carrega config antes de qualquer comando
│   ├── init.go  serve.go  seed.go
│   ├── events.go  metrics.go  db.go  import.go
├── internal/
│   ├── config/config.go          # .env + variáveis de ambiente, validação
│   ├── core/
│   │   ├── domain/               # entidades e enums (sem dependências externas)
│   │   ├── ports/                # interfaces dos repositórios
│   │   └── services/             # casos de uso (+ testes com fakes)
│   └── adapters/
│       ├── http/                 # server.go, event_handler.go, dto.go
│       └── repository/           # PostgreSQL (pgx/v5 via database/sql)
├── migrations/                   # SQL + embed.go, aplicados por `tracelog migrate`
├── docker-compose.yml            # serviço postgres:16-alpine
├── .env.example
└── docs/                         # documentação
```

Fluxo de dependência: `cmd`/`http` → `services` → `ports` ← `repository`. O domínio não depende de nada externo.

Dependências diretas (`go.mod`): `github.com/jackc/pgx/v5` e `github.com/spf13/cobra`.

---

## 10. Como estender

- **Novo caso de uso:** `internal/core/services/` + teste com fake do repositório.
- **Novo método de persistência:** declarar em `internal/core/ports/`, implementar em `internal/adapters/repository/`.
- **Nova rota HTTP:** registrar em `internal/adapters/http/server.go`.
- **Novo comando CLI:** arquivo em `cmd/` com `rootCmd.AddCommand(...)` no `init()`.
- **Nova migration:** `migrations/00000N_descricao.sql`, **idempotente** (`IF NOT EXISTS`), porque o script reaplica tudo.
- **Commits:** Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `test:`), como no histórico atual.

---

## 11. Limitações conhecidas

1. **Migrations sem *down*:** correção de schema é sempre uma migration nova.
2. **Truncagem de `external_id`**: dois valores longos com os mesmos 255 primeiros caracteres viram o mesmo evento (risco aceito na spec 0002).
3. **Sem validação de `message`** no HTTP: qualquer valor é aceito e normalizado.
4. **Sem autenticação, rate limiting ou versionamento** de API (`/v1`). Documentado como futuro.
5. **Heurística de severidade** não entende negação (seção 7.3).
6. **`seed` não popula nada.**
7. **`--help` exige `.env` válido.**
8. O CI (`.github/workflows/ci.yml`) roda gofmt, `go vet`, `go test -race` com PostgreSQL e build, além de um job que constrói a imagem Docker e confere que ela não roda como root.

---

## 12. Divergências conhecidas na documentação

Os arquivos abaixo ainda descrevem comportamentos que não existem. Corrija-os antes de confiar neles.

| Documento | Afirmação | Realidade no código |
|-----------|-----------|---------------------|
| `README.md`, `docs/getting-started.md` | Go 1.25+ | `go.mod` declara 1.26.11 |
| `docs/development.md` | Go 1.21+ | `go.mod` declara 1.26.11 |
| `docs/http-api.md`, `docs/development.md` | `severity` obrigatório/enviado no `POST /events` | Ignorado; severidade é derivada |
| `docs/http-api.md` | `400` para campos ausentes, `message is required`, JSON inválido | Não existe; dados são normalizados e gravados com `201` |
| `docs/cli.md`, `docs/getting-started.md` | `seed` popula ambientes, apps e fontes | `seed` só imprime mensagem |
| `docs/architecture.md`, `docs/domain.md` | Tabelas `technical_events`, `monitored_applications`, `environments`, `event_sources` | Nomes reais: `technical_event`, `monitored_application`, `environment`, `event_source` |
| `docs/development.md` | Migration `000002_bootstrap_data.sql`; seeds nas migrations | Não existe; a `000002` é `add_log_import_fields`; seeds removidos (`484fea9`) |
| `docs/development.md` | `scripts/setup-demo-data.sh`, `scripts/seed-error-data.sh` | Não existem |
| `docs/architecture.md` | Driver `github.com/lib/pq` | Driver real: `pgx/v5` (stdlib) |
| `docs/development.md` | `.env` na raiz e `go run . serve` funcionam de qualquer pasta | `.env` é lido do diretório atual |
| `docs/domain.md` | Canal `file` "futuro" | Implementado (`import logs`) |
| `docs/cli.md` | Colunas de `events list`: `MESSAGE` antes de `SEVERITY` | Ordem real: `OCCURRED_AT SEVERITY APPLICATION_ID ENVIRONMENT_ID MESSAGE` |

---

## 13. Próximos passos sugeridos

Itens não implementados ou que merecem decisão antes de avançar:

1. **Corrigir a documentação** conforme a seção 12 (ou consolidar `docs/` em torno deste guia).
2. **Controle de migrations**: tabela de versões no banco ou ferramenta dedicada, em vez de reaplicar tudo.
3. **Lote e `external_id` obrigatório** (spec 0001, D3 e D5), adiados para depois da v0.1.0.
4. **Testes de ponta a ponta da CLI** contra banco (`init` → `POST /events` → `events list`).
5. **Validação de entrada HTTP** (ex.: `message` obrigatória) versus a política atual de ingestão tolerante.
6. **Ferramentas de DX**: `Makefile` com `build`, `test`, `vet`, `migrate`, ou documentar a ausência.
7. **Integração RabbitMQ** (`internal/adapters/amqp/`), conforme `docs/architecture.md`.

---

**Última atualização deste guia:** fase 2, spec 0002 (2026-11-20).
