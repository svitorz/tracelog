# Guia do Projeto TraceLog

Documento de referência para retomar o desenvolvimento do TraceLog. Descreve o estado atual do código, como instalar, compilar, executar e testar, e o que ainda precisa de atenção.

> Este guia foi escrito a partir do código no commit `456c830`. Quando algo aqui divergir do código, o código prevalece. As divergências conhecidas com os outros documentos de `docs/` estão listadas na seção [Divergências conhecidas](#divergências-conhecidas-na-documentação).

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
| Go | **1.24.4+** (conforme `go.mod`) | Os docs antigos citam 1.21 e 1.25; a referência válida é o `go.mod`. |
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
| `DB_HOST` | não | `localhost` | Host do PostgreSQL. |
| `DB_PORT` | não | `5432` | Porta do PostgreSQL; também é a porta publicada pelo Docker. |
| `DB_NAME` | **sim** | — | Nome do banco. |
| `DB_USER` | **sim** | — | Usuário do banco. |
| `DB_PASSWORD` | **sim** | — | Senha do banco. |
| `DB_SSLMODE` | não | `disable` | Um de `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`. |

**Precedência:** variável de ambiente do shell > `.env`. Um `.env` nunca sobrescreve uma variável já definida.

> **Atenção:** sem `DB_NAME`, `DB_USER` e `DB_PASSWORD`, **qualquer** comando do binário falha antes de executar, inclusive `--help`.

### 4.3 Subir o PostgreSQL

```bash
docker compose up -d postgres
docker compose ps
```

O serviço (`docker-compose.yml`) usa `postgres:16-alpine`, container `tracelog-postgres`, volume `tracelog_postgres_data` e healthcheck com `pg_isready`.

Para zerar o banco (apaga os dados):

```bash
docker compose down -v
```

### 4.4 Aplicar as migrations

```bash
./scripts/run-migrations.sh
```

O script:

1. cria o `.env` a partir do `.env.example` se ele não existir;
2. carrega o `.env`;
3. sobe o serviço `postgres`, se necessário;
4. executa **todos** os `migrations/*.sql` em ordem alfabética, com `psql -v ON_ERROR_STOP=1`.

Migrations atuais:

| Arquivo | Conteúdo |
|---------|----------|
| `000001_initial_schema.sql` | Tabelas `environment`, `monitored_application`, `event_source`, `technical_event`, FKs e índices. |
| `000002_add_log_import_fields.sql` | Colunas `raw_line`, `normalization_status`, `parse_error` em `technical_event` e índice. |

> **Importante:** não há controle de versão. Cada execução reaplica todos os arquivos. Por isso cada migration usa `IF NOT EXISTS`. Qualquer migration nova precisa seguir o mesmo padrão idempotente, ou o script precisa ser evoluído para registrar o que já foi aplicado.

### 4.5 Setup inicial (obrigatório antes do `serve`)

```bash
go run . init
```

É interativo. Pressione Enter para aceitar os padrões:

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

Não há `Makefile` no projeto. Os comandos acima são o equivalente.

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

Para encerrar: `Ctrl+C` (SIGINT). Observação: o servidor usa `http.ListenAndServe` direto, sem *graceful shutdown*. Os docs que falam em shutdown gracioso estão imprecisos.

---

## 6. Uso

### 6.1 API HTTP

Base: `http://localhost:8080` (ou a porta configurada).

| Método | Rota | Função |
|--------|------|--------|
| `GET` | `/health` | Retorna `{"status":"ok"}`. Não checa o método. |
| `POST` | `/events` | Registra um evento. Retorna `201 Created`. |

Corpo do `POST /events`:

| Campo | Tipo | Obrigatório | Comportamento real |
|-------|------|-------------|--------------------|
| `message` | string | não | Se vazio, grava `event received without message`. |
| `occurred_at` | string | não | RFC3339 (com ou sem fração/offset) ou `YYYY-MM-DD HH:MM:SS`. Ausente/inválido → usa o horário de recebimento. |
| `category` | string | não | `application`, `system`, `security`, `audit`, `unknown`. Ausente/inválido → `unknown`. |
| `payload` | objeto | não | JSON livre. |
| `external_id` | string | não | Vazio vira `NULL`. Deve ser único. |
| `trace_id` | string | não | |
| `correlation_id` | string | não | |
| `application_id` | UUID | não* | Ausente/inválido → setup do `init`. |
| `source_id` | UUID | não* | Ausente/inválido → setup do `init`. |
| `environment_id` | UUID | não* | Ausente/inválido → setup do `init`. |

\* Na prática são opcionais, desde que o `init` tenha sido executado (há fallback). Use sempre os IDs retornados pelo `init` para não depender do fallback.

**Campos que não existem mais no contrato:** `severity` é **ignorado** se enviado. O servidor deriva a severidade (seção 7.3).

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

Resposta (`201`):

```json
{
  "id": "<uuid>",
  "external_id": "",
  "received_at": "2026-03-09T14:30:01.234Z",
  "created_at": "2026-03-09T14:30:01.234Z"
}
```

Erros:

| Status | Quando |
|--------|--------|
| `405` | Método diferente de `POST` em `/events`. |
| `500` | Falha ao persistir (ex.: banco fora do ar, violação de unicidade de `external_id`). |

JSON malformado **não** gera erro: o evento é gravado com `payload._ingestion.decode_error`.

### 6.2 CLI

Com o binário compilado, substitua `go run .` por `tracelog`.

| Comando | Função | Flags principais |
|---------|--------|------------------|
| `serve` | Sobe o HTTP | `--http-port` |
| `init` | Setup inicial interativo | — |
| `events list` | Lista eventos (mais recentes primeiro) | `--severity`, `--application-id`, `--environment-id`, `--from`, `--to`, `--limit` (padrão 50) |
| `metrics errors` | Contagem de erros por dia | `--application-id`, `--environment-id`, `--from`, `--to` |
| `db list environments\|applications\|sources` | Lista entidades de referência com IDs | — |
| `import logs` | Importa arquivos de log | `--file` (obrigatório, repetível) |
| `seed` | **No-op** (apenas imprime mensagem) | — |

Formatos de data em `--from`/`--to`: `2026-03-09` (00:00 UTC) ou RFC3339 (`2026-03-09T14:30:00Z`).

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

Ordem de avaliação (a primeira regra que casar vence):

1. `payload.level` com valor reconhecido (`trace`/`debug`, `info`, `warn`/`warning`, `error`/`err`, `fatal`/`critical`/`panic`).
2. Mensagem contém `panic`, `fatal`, `unrecoverable` ou `crash`, **ou** payload tem `panic`, `fatal` ou `stack_trace` com valor significativo → `fatal`.
3. Mensagem contém `error`, `failed`, `failure`, `timeout`, `exception`, `unauthorized` ou `denied`, **ou** payload tem `error`, `errors`, `exception` ou `exception_message` → `error`.
4. Categoria `security`, **ou** mensagem contém `warn`, `warning`, `retry`, `deprecated`, `slow` ou `fallback`, **ou** payload tem `warning`, `warnings` ou `retry_count` → `warn`.
5. Caso contrário → `info`.

### 7.4 Métricas

`metrics errors` conta apenas `severity = 'error'`, agrupado por dia. Eventos `fatal` **não** entram na contagem (ver divergências).

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

Estado no commit `456c830`: `go vet ./...` limpo e `go test ./...` passando.

### 8.2 Cobertura atual dos testes

| Pacote | Testes | O que cobre |
|--------|--------|-------------|
| `internal/core/services` | 12 | Derivação de severidade, fallback de IDs e retry após FK, importação (multi-formato, JSON quebrado), `init` (normalização e campos obrigatórios), `EnsureSystemSetup`, listagem com normalização de severidade. |
| `internal/config` | 3 | Defaults, variáveis obrigatórias, `.env` sem sobrescrever o ambiente. |
| `internal/adapters/repository` | 1 | Mapeamento amigável de erro de FK. |
| `internal/adapters/http`, `cmd`, `internal/core/domain` | 0 | Sem testes. |

Os testes de `services` usam repositórios em memória (fakes) e **não** tocam o banco.

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

- Não há testes de integração contra PostgreSQL real (o `docs/development.md` descreve um banco `tracelog_test`, mas nenhum teste o usa).
- Não há testes do handler HTTP (`EventHandler`, `Server`).
- Não há testes dos comandos da CLI.

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
├── migrations/                   # SQL aplicado por scripts/run-migrations.sh
├── scripts/run-migrations.sh
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

1. **Migrations sem controle de versão** (seção 4.4).
2. **`external_id` duplicado** gera `500` (violação da constraint `uq_technical_event_external_id`). Não há tratamento específico nem resposta `409`. Não verificado em execução.
3. **Sem validação de `message`** no HTTP: qualquer valor é aceito e normalizado.
4. **Sem autenticação, rate limiting ou versionamento** de API (`/v1`). Documentado como futuro.
5. **Sem graceful shutdown** no servidor.
6. **Erros de validação do HTTP são 500**, não 400, porque o caso de uso só rejeita canal inválido (que o HTTP nunca envia).
7. **`seed` não popula nada.**
8. **`--help` exige `.env` válido.**
9. **Métricas ignoram `fatal`.**
10. Não há Makefile nem CI configurada.

---

## 12. Divergências conhecidas na documentação

Os arquivos abaixo ainda descrevem comportamentos que não existem. Corrija-os antes de confiar neles.

| Documento | Afirmação | Realidade no código |
|-----------|-----------|---------------------|
| `README.md`, `docs/getting-started.md` | Go 1.25+ | `go.mod` declara 1.24.4 |
| `docs/development.md` | Go 1.21+ | `go.mod` declara 1.24.4 |
| `docs/http-api.md`, `docs/development.md` | `severity` obrigatório/enviado no `POST /events` | Ignorado; severidade é derivada |
| `docs/http-api.md` | `400` para campos ausentes, `message is required`, JSON inválido | Não existe; dados são normalizados e gravados com `201` |
| `docs/cli.md`, `docs/getting-started.md` | `seed` popula ambientes, apps e fontes | `seed` só imprime mensagem |
| `docs/architecture.md`, `docs/domain.md` | Tabelas `technical_events`, `monitored_applications`, `environments`, `event_sources` | Nomes reais: `technical_event`, `monitored_application`, `environment`, `event_source` |
| `docs/development.md` | Migration `000002_bootstrap_data.sql`; seeds nas migrations | Não existe; a `000002` é `add_log_import_fields`; seeds removidos (`484fea9`) |
| `docs/development.md` | `scripts/setup-demo-data.sh`, `scripts/seed-error-data.sh` | Não existem |
| `docs/architecture.md` | Driver `github.com/lib/pq` | Driver real: `pgx/v5` (stdlib) |
| `docs/development.md` | `.env` na raiz e `go run . serve` funcionam de qualquer pasta | `.env` é lido do diretório atual |
| `docs/domain.md` | Métricas consideram `error` e `fatal` | Apenas `error` |
| `docs/domain.md` | Canal `file` "futuro" | Implementado (`import logs`) |
| `docs/cli.md` | Colunas de `events list`: `MESSAGE` antes de `SEVERITY` | Ordem real: `OCCURRED_AT SEVERITY APPLICATION_ID ENVIRONMENT_ID MESSAGE` |
| `docs/cli.md` | `serve` com shutdown gracioso | Sem graceful shutdown |

---

## 13. Próximos passos sugeridos

Itens não implementados ou que merecem decisão antes de avançar:

1. **Corrigir a documentação** conforme a seção 12 (ou consolidar `docs/` em torno deste guia).
2. **Controle de migrations**: tabela de versões no banco ou ferramenta dedicada, em vez de reaplicar tudo.
3. **Tratar `external_id` duplicado** com resposta clara (ex.: `409`) e decidir se deve ser idempotente.
4. **Testes de integração** com PostgreSQL (repositórios e fluxo `init` → `POST /events` → `events list`).
5. **Testes do handler HTTP** (`httptest`) para método, JSON inválido e status codes.
6. **Decidir sobre `fatal` em `metrics errors`.**
7. **Validação de entrada HTTP** (ex.: `message` obrigatória) versus a política atual de ingestão tolerante.
8. **Graceful shutdown** do servidor (`signal.NotifyContext` + `http.Server.Shutdown`).
9. **Ferramentas de DX**: `Makefile` com `build`, `test`, `vet`, `migrate`, ou documentar a ausência.
10. **Integração RabbitMQ** (`internal/adapters/amqp/`), conforme `docs/architecture.md`.

---

**Última atualização deste guia:** commit `456c830` (2026-10-15).
