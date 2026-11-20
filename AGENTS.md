# AGENTS.md

Instruções para agentes de IA (e para contribuidores humanos) que trabalham neste repositório. Leia antes de alterar qualquer arquivo.

## 1. O que é o projeto

TraceLog é uma ferramenta de observabilidade open source (GPL-3.0) para **ingestão, armazenamento e investigação de logs e eventos técnicos**. Ela deve ser fácil de instalar e adaptável a diferentes modos de uso: servidor local para um time, instalação por máquina e servidor central recebendo de apps desktop.

**Escopo atual do MVP:** ingestão (HTTP e arquivos), persistência e consulta por CLI/SQL.
**Fora do escopo por enquanto:** autenticação, login, interface web, app desktop, dashboards. Não implemente nada disso sem uma spec aprovada.

Roadmap em ordem: (1) fundação OSS, (2) núcleo de investigação (pipe, busca, agrupamento por fingerprint, visão por `trace_id`), (3) SQLite como padrão com PostgreSQL opcional, (4) API JSON e UI web embutida, (5) MCP e busca em linguagem natural, (6) dashboards.

## 2. Stack e comandos

- Go **1.26.11** (`go.mod`), módulo `svitorz/tracelog`.
- Dependências diretas: `github.com/jackc/pgx/v5` (driver via `database/sql`), `github.com/spf13/cobra`.
- PostgreSQL 16 via `docker-compose.yml`.

```bash
cp .env.example .env                    # primeira vez
docker compose up -d                    # banco + migrate + init --defaults + API

# ou, com a CLI no host:
docker compose up -d postgres           # só o banco
go run . migrate                        # aplica migrations pendentes
go run . init                           # setup obrigatório antes do serve
go run . serve                          # servidor HTTP

go build -o bin/tracelog .              # binário
go vet ./...
go test ./...
gofmt -l .                              # deve sair vazio
```

Rode `go vet ./...` e `go test ./...` antes de considerar uma tarefa concluída.

## 3. Arquitetura (ports/adapters)

```
main.go → cmd/ (CLI) ──┐
internal/adapters/http ─┴→ internal/core/services → internal/core/ports ← internal/adapters/repository
                              ↓
                       internal/core/domain
```

Regras:

- `internal/core/domain` não importa nada externo (nem pacotes de `adapters`, `ports` ou `services`, nem drivers).
- `internal/core/services` depende só de `domain` e `ports`. Não importe `database/sql`, `net/http` ou Cobra aqui.
- Adaptadores (`cmd/`, `internal/adapters/...`) implementam ou usam as portas. Não coloque regra de negócio neles.
- Toda interação externa passa por uma interface em `internal/core/ports/`.
- Testes de casos de uso usam **fakes em memória**, não banco real.

A camada de repositório hoje é específica de PostgreSQL (`JSONB`, `gen_random_uuid()`, `pgcrypto`, casts `::uuid`). Trocar o banco exige reescrever essa camada, não só o driver.

## 4. Regras de domínio e de dados (não violar)

- **`severity` não é confiável vinda do cliente no HTTP.** É derivada no servidor (`deriveSeverity` em `register_technical_event_usecase.go`). Qualquer mudança nessa regra precisa de spec e testes.
- **Ingestão tolerante:** campos ausentes ou inválidos são normalizados e anotados em `payload._ingestion`. O erro HTTP só deve ocorrer quando nada pode ser persistido. Mudanças nessa política precisam de spec.
- **Timestamps:** `occurred_at` é o horário reportado pelo cliente (pode estar errado em máquinas de usuários); `received_at` é definido pelo servidor. Não sobrescreva um pelo outro.
- **IDs de referência com fallback:** application, source e environment inválidos caem no setup do `init`. Isso é intencional, mas deve ser anotado no payload.
- `serve` e `import` exigem o setup (`tracelog init`). Não remova essa verificação sem spec.

## 5. Migrations

- Arquivos em `migrations/`, nomeados `NNNNNN_descricao.sql`, embutidos no binário e aplicados em ordem por `tracelog migrate`, que registra cada versão em `schema_migrations` (spec 0003).
- **Toda migration continua tendo que ser idempotente** (`CREATE ... IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `DROP ... IF EXISTS`): se o processo morrer entre aplicar e registrar, o arquivo roda de novo, e bancos migrados pelo antigo script são adotados reaplicando tudo.
- Cada arquivo controla a própria transação (`BEGIN`/`COMMIT`).
- Nunca edite uma migration já commitada. Crie uma nova.

## 6. Configuração

- Carregada em `internal/config/config.go` **antes** de qualquer comando do Cobra. Por isso `--help` falha sem `DB_NAME`, `DB_USER` e `DB_PASSWORD`.
- `.env` é lido do **diretório atual** (caminho relativo). Os comandos devem rodar a partir da raiz do repositório.
- Precedência: variável de ambiente > `.env`.
- Nunca commite `.env`. Use `.env.example` para novas variáveis e documente-as no guia.

## 7. Specs e decisões

- Mudanças que alteram **contrato** (rotas HTTP, formato de resposta, comandos da CLI, schema, configuração) exigem uma spec em `docs/specs/` antes da implementação.
- Formato da spec: `docs/specs/NNNN-titulo-curto.md` com: problema, objetivos, não objetivos, comportamento, contrato, erros, mudanças de schema, critérios de aceite e questões em aberto.
- Decisões de arquitetura importantes viram ADR em `docs/adr/` (criar quando o primeiro ADR for necessário).
- Specs e ADRs são a fonte de verdade para o "porquê". Se o código contradizer uma spec aceita, atualize uma ou outra explicitamente.

## 8. Documentação

- `docs/guia-do-projeto.md` é o guia de referência do estado atual. **Atualize-o** quando um comportamento, comando ou variável mudar.
- Os demais arquivos em `docs/` (`cli.md`, `http-api.md`, `domain.md`, `architecture.md`, `development.md`, `getting-started.md`) têm divergências conhecidas listadas na seção 12 do guia. Não copie afirmações deles sem verificar no código.
- Não documente funcionalidade que não existe. Se algo for planejado, diga que é planejado.

## 9. Testes

- Cada caso de uso novo ou alterado precisa de teste em `internal/core/services/`, com fake do repositório.
- Adaptadores HTTP e CLI não têm testes hoje. Ao alterá-los, adicione testes quando for viável (`httptest` para HTTP).
- Testes de integração com PostgreSQL ainda não existem. Se você criar um, deixe-o explícito (ex.: build tag ou variável de ambiente) para que `go test ./...` continue funcionando sem banco.
- Nunca altere um teste para fazê-lo passar sem entender por que ele falhou.

## 10. Convenções de código e Git

- Código Go idiomático: `gofmt`, erros com contexto (`fmt.Errorf("...: %w", err)`), sem `panic` em caminho de requisição.
- Nomes de identificadores, pacotes e arquivos em **inglês**. Mensagens de usuário e documentação nova em **português**, seguindo o arquivo em que você está.
- Comentários só quando o comportamento não for óbvio pelo código. Acompanhe a densidade de comentários do arquivo.
- Commits no padrão Conventional Commits, em português ou inglês, como no histórico: `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`.
- Um commit por mudança lógica. Não misture refatoração com comportamento novo.
- Não commite `bin/`, `.env`, arquivos de cobertura ou logs (já estão no `.gitignore`).

## 11. Pendências conhecidas

Lista completa e atualizada em `docs/guia-do-projeto.md`, seções 11 e 13. Resumo dos mais importantes:

- `external_id` duplicado retorna `500` (ver spec de ingestão em lote).
- Sem limite de tamanho de corpo no HTTP.
- Sem graceful shutdown.
- Importação de arquivos faz um `INSERT` por linha, sem lote.
- `metrics errors` ignora `fatal`.
- `seed` é no-op.

## 12. O que pedir confirmação antes de fazer

- Mudar a licença ou adicionar cabeçalhos de licença em arquivos.
- Adicionar dependências novas.
- Alterar o schema de forma destrutiva (`DROP TABLE`, remover coluna, apagar dados).
- Adicionar autenticação, interface web ou qualquer item fora do escopo do MVP.
- Executar comandos que alterem o banco de dados do usuário (`docker compose down -v`, migrations contra banco real).
