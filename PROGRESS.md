# PROGRESS

Memória de trabalho do preparo do TraceLog para portfólio (v0.1.0). Leia este arquivo antes de qualquer coisa ao retomar uma sessão.

## Fase atual

**Trilha 1 encerrada.** Toda a cadeia foi mesclada no `main` pelos PRs #2, #3 e #4 (merge `492f362`), com CI verde, e a tag anotada `v0.1.0` aponta para `492f362`. **Trabalho em andamento: Trilha 2** (seção "Trilha 2: API de leitura e dashboard", no fim deste arquivo).

## Checklist

| Fase | Branch | Status |
|------|--------|--------|
| 0. Orientação | `chore/ci-baseline` | concluída |
| 1. Rede de segurança e CI | `chore/ci-baseline` | concluída; PR #1 mergeado; CI verde no PR e no push para `main` |
| 2. Robustez da API HTTP | `fix/http-hardening` | concluída; push feito, sem PR (CI não rodou) |
| 3. Container e migrations | `feat/container` | concluída (sem push; depende de `fix/http-hardening`) |
| 4. Dados de exemplo e demo | `feat/sample-data` | concluída (sem push; depende de `feat/container`); GIF pendente com o autor |
| 5. Medição de banco (opcional) | — | só se pedido |
| 6. Auditoria da documentação | `docs/audit` | concluída (sem push) |
| 7. Release v0.1.0 | `release/v0.1.0` | concluída; tag e release com o autor |

## Decisões pendentes

1. ~~**Licença.**~~ Decidido: GPL-3.0 (ver log de decisões).
2. ~~**Spec 0001 x prompt.**~~ Decidido: só dedupe na v0.1.0 (ver log de decisões). Texto original: `docs/specs/0001-ingestao-em-lote-idempotente.md` (não commitada, marcada "Aprovada") já decide H4: unicidade `(application_id, external_id)`, duplicata responde `200` com `"duplicate": true`, `external_id` obrigatório no HTTP (400) e cria `POST /events/batch`. Isso conflita com "decida comigo 409 ou devolver existente", com H5 ("não perder evento": sem `external_id` decodificável a requisição viraria 400) e com o escopo enxuto da Fase 2.
3. ~~**Arquivos não commitados.**~~ Decidido: commitados na Fase 1, um commit por arquivo lógico.
4. ~~**Makefile.**~~ Decidido: criar na Fase 3 (ver log de decisões).
5. ~~**Versão do Go.**~~ Decidido: 1.26.11 (ver log de decisões).
6. ~~**`pgcrypto` (H13).**~~ Decidido: deixar e documentar (spec 0003 §9, guia §4.4). Contexto: não removido porque `000001` (já commitada, não pode ser editada) cria a extensão em todo banco novo; tirá-la exigiria `DROP EXTENSION IF EXISTS pgcrypto` numa `000004`, ou seja, criar e apagar em toda instalação, e é mudança destrutiva de schema (AGENTS.md pede confirmação). Desde o PostgreSQL 13 a `pgcrypto` é extensão *trusted*, então não exige superusuário. Recomendação: deixar e documentar como inofensiva. Na pergunta da fase 3 eu tinha dito que a removeria; revi ao implementar.

## Log de decisões

| Data | Decisão | Porquê |
|------|---------|--------|
| 2026-11-20 | Licença GPL-3.0, não MIT. Na Fase 1, o item "adicionar LICENSE MIT" vira "commitar o `LICENSE` GPL-3.0 e o README já ajustados" | Decisão do autor; o working tree e o `AGENTS.md` já estavam em GPL-3.0 |
| 2026-11-20 | Spec 0001 na v0.1.0: só a parte de duplicidade. Unicidade `(application_id, external_id) WHERE external_id IS NOT NULL` por migration nova; duplicata responde `200` com `"duplicate": true` e o evento existente. Ficam para depois da v0.1.0: `external_id` obrigatório (D5) e `POST /events/batch` (D3). A spec será atualizada na Fase 2 para registrar o recorte | Mantém a Fase 2 pequena e preserva a política de ingestão tolerante (H5): sem `external_id`, o evento continua sendo gravado. Descartado: 409, que obrigaria o cliente a tratar erro num reenvio que deu certo |
| 2026-11-21 | CHANGELOG em inglês (Keep a Changelog), cobrindo desde a tag existente `v0.0.1` (`72aa48e`, 2026-03-09) e marcando o que quebra compatibilidade | Mesma língua do README; `v0.0.1` já está publicada, então a v0.1.0 a compara com ela |
| 2026-11-21 | Sem flag `--version` no binário | Não pedida e não necessária para a release; a versão fica na tag e no CHANGELOG |
| 2026-11-20 | Fase 6: apagar os 6 documentos antigos de `docs/`; ficam README, guia, specs, AGENTS | Decisão do autor. Repetiam o guia e tinham dezenas de afirmações falsas; uma fonte só. Descartados: manter `http-api.md`/`cli.md` reescritos; reescrever os 6 |
| 2026-11-20 | README em inglês, guia em português | Decisão do autor: alcance no GitHub; o README aponta para o guia |
| 2026-11-20 | Guia reescrito inteiro em vez de remendado | Acumulava remendos de 4 fases (cabeçalho duplicado, numeração pulada, referências mortas); reescrever permitiu conferir cada afirmação |
| 2026-11-20 | Saídas nos docs só copiadas de capturas (`demo-output.txt`, logs de teste) | Regra 10 do prompt; o README é montado lendo `docs/demo-output.txt` |
| 2026-11-20 | Fase 4 guiada pela spec 0004 (`docs/specs/0004-importacao-idempotente-e-dados-de-exemplo.md`) | Mudanças de CLI (remove `seed`, resumo do import) e de comportamento do import |
| 2026-11-20 | `seed` removido | Dados de referência vêm do `init`; demonstração vem de `examples/logs` + `import logs`. Descartado: `seed` com dataset embutido (duplica o import, incha o binário) |
| 2026-11-20 | Reimportação: `external_id = file:` + sha256(nome base, linha, conteúdo)[:32] | Usa a deduplicação da fase 2; nome base para host e container gerarem o mesmo ID. Descartado: só documentar |
| 2026-11-20 | Categorizador: segurança antes de audit, palavra inteira | `login failed`/`unauthorized` caíam em audit; ramo security inalcançável |
| 2026-11-20 | Linhas em branco puladas e contadas | Não têm informação. Descartado: gravar como uncategorizable |
| 2026-11-20 | Erros do import: até 10 exemplos `arquivo:linha: motivo` | Operador precisa achar a linha; o motivo completo pode ir à CLI (diferente do HTTP) |
| 2026-11-20 | Linha com `[` só é JSON se for JSON válido | Achado com o dataset: `[06/Mar/2026:...] WARN` virava texto puro sem nível |
| 2026-11-20 | Categorizador não promove `partial` → `normalized` | Achado com o dataset: linha com timestamp ilegível aparecia como normalized |
| 2026-11-20 | Gerador em Go (`tools/loggen`), dataset versionado = saída dele, teste de igualdade | Sem dependência, reproduzível entre SOs, coberto por vet/CI. Descartado: script shell (`$RANDOM`/`date` variam) |
| 2026-11-20 | Import com `synchronous_commit=off`, mantendo um INSERT por linha | Medição mostrou problema real (fsync = ~90% do tempo). Descartados: transações em lote (port transacional), manter como está |
| 2026-11-20 | H10 corrigido (aspas e escape no DSN) | Estava confirmado desde a fase 0 e eu ia mexer no DSN; teste com `pgconn.ParseConfig` + senha real `a b'c` |
| 2026-11-20 | Demo usa a CLI dentro do container (`docker compose run`, exemplos em volume `:ro`) | Exige só Docker e curl, sem Go no host |
| 2026-11-20 | Fase 3 guiada pela spec 0003 (`docs/specs/0003-container-e-migrations.md`) | Novo comando, nova flag, nova tabela e compose mudam contrato |
| 2026-11-20 | Migrations: `tracelog migrate` com `go:embed` + `schema_migrations` + `pg_advisory_lock` | Sem dependência nova, funciona em container e CI. Descartados: goose/golang-migrate (dependência e peso), serviço psql (só resolve o compose) |
| 2026-11-20 | Migrator no adapter de repositório, sem port/serviço | Migração é infraestrutura, não regra de negócio; um port só para isso seria camada a mais. SQL continua só no adapter |
| 2026-11-20 | Cada arquivo roda com a própria transação, registro logo depois (não atômico) | As migrations commitadas têm `BEGIN`/`COMMIT` e não podem ser editadas; embrulhá-las em outra transação faria o `COMMIT` interno encerrar a externa. Custo: morte entre aplicar e registrar reaplica o arquivo, coberto pela regra de idempotência |
| 2026-11-20 | Orquestração: serviços one-shot `migrate` → `init` → `api` com `service_completed_successfully` | Passos visíveis e com falha isolada. Descartado: entrypoint único |
| 2026-11-20 | `init --defaults` (uma flag) | Contrato mínimo. Descartado: 9 flags, uma por campo |
| 2026-11-20 | Runtime `alpine:3.24` + `wget` no healthcheck, UID 10001 | Sem código novo para healthcheck. Descartado: distroless + subcomando `healthcheck` |
| 2026-11-20 | Imagens fixadas: `golang:1.26.11-alpine3.24`, `alpine:3.24`, `postgres:16.16-alpine` | Build reproduzível; tags conferidas na API do Docker Hub |
| 2026-11-20 | Portas só em `127.0.0.1`; `DB_HOST`/`DB_PORT` do container por `environment:`; sem `env_file` | H11: a app no container não herda `DB_HOST=localhost`; só as variáveis necessárias entram no container |
| 2026-11-20 | Removido `container_name: tracelog-postgres` | Permite mais de um projeto compose na mesma máquina (usado no teste com `-p tracelog-phase3`) |
| 2026-11-20 | `scripts/run-migrations.sh` removido (confirmado na escolha da opção); docs passam a citar `tracelog migrate` | Duas formas de migrar divergiriam |
| 2026-11-20 | CI ganhou job `docker build` + checagem de UID | Dockerfile quebrado seria descoberto só pelo usuário |
| 2026-11-20 | Fase 2 guiada pela spec 0002 (`docs/specs/0002-endurecimento-da-api-http.md`), escrita antes do código | `AGENTS.md` exige spec para mudança de contrato (rotas, status, config, schema, saída da CLI) |
| 2026-11-20 | H7: palavra inteira + flexões + CamelCase `*Error`/`*Exception` | Remove falsos positivos de substring ("terror", "crashpad"); negação continua não tratada e está documentada em teste. Descartado: manter substring |
| 2026-11-20 | H8: colunas `ERRORS`, `FATAL`, `TOTAL`; dia em UTC | Nada some e a distinção fica visível. Descartado: somar fatal em uma coluna; manter só error |
| 2026-11-20 | H6: `/health` liveness sem banco + `/ready` com ping de 2s | Banco lento não faz o orquestrador matar processo saudável. Descartado: só `/health` checando banco |
| 2026-11-20 | `Content-Type` explícito não JSON → 415; ausente aceito | Decisão do autor (eu havia recomendado não obrigatório). Consequência: `curl -d` sem `-H 'Content-Type: application/json'` manda form-urlencoded e recebe 415 |
| 2026-11-20 | `payload.level` ignorado para severidade no canal http; vale no file | Cumpre a regra "severity não é confiável vinda do cliente" sem perder o nível das linhas de log |
| 2026-11-20 | Campos maiores que a coluna são truncados por caractere e anotados | Decisão do autor (eu havia recomendado 422). Risco aceito: `external_id` longos com o mesmo prefixo de 255 viram duplicata |
| 2026-11-20 | Mapeamento de erros: só 405, 413, 415 e 500 genérico; sem 400/409/422 | Com ingestão tolerante, dedupe em 200 e truncagem, nenhum caso atual produz 400/409/422; criar erros tipados para eles seria código morto. Único erro tipado: `ports.ErrReferenceNotFound` (H9) |
| 2026-11-20 | Timeouts: header 5s, read 15s, write 15s, idle 60s, handler 10s, shutdown 8s, `/ready` 2s | Justificativa na spec 0002 §4.1; shutdown abaixo dos 10s do Docker antes do SIGKILL |
| 2026-11-20 | Pool: 10 abertas, 10 ociosas, vida 30m, ociosidade 5m | Teto contra esgotar `max_connections` (100); constantes no código, sem nova variável de ambiente |
| 2026-11-20 | Testes de integração opcionais via `TRACELOG_TEST_DATABASE_URL`, schema descartável por teste, com `timezone=America/Sao_Paulo` | `go test ./...` continua sem banco (AGENTS.md §9); schema próprio permite apontar para qualquer banco sem tocar tabelas; fuso não-UTC prova a agregação em UTC. CI ganhou serviço `postgres:16-alpine` |
| 2026-11-20 | Saída de `metrics errors` toda em stdout | Corrige parte do H18 no código que já estava sendo reescrito; o resto do H18 (`cmd/db.go`, demais comandos) continua pendente |
| 2026-11-20 | `go.mod`: `go 1.26.11` (era 1.24.4) | 1.24 fora de suporte; 1.26 é suportada. Patch explícito para o CI e o `GOTOOLCHAIN=auto` local usarem a mesma versão; o custo é atualizar o patch manualmente. Descartado: `go 1.26` sem patch (versão efetiva menos previsível) e 1.27 (mais nova, sem necessidade) |
| 2026-11-20 | Makefile mínimo criado só na Fase 3 (`test`, `lint`, `up`, `migrate`, `demo`); CI continua chamando `go` direto | Antes disso os alvos `up`/`migrate`/`demo` seriam vazios ou falsos. CI direto deixa claro no log qual passo falhou. Descartado: sem Makefile (perde o atalho e a descoberta de comandos) |
| 2026-11-20 | `go.mod` não foi alterado | Não há divergência: `go.mod` = 1.24.4 = Go local. As menções a 1.25+/1.21+ estão erradas nos docs (Fase 6). A atualização de versão ficou como decisão pendente 5 |
| 2026-11-20 | CI em um único job (`ubuntu-24.04`) com gofmt, `go vet`, `go test -race -count=1`, `go build`; `actions/checkout@v7.0.1` e `actions/setup-go@v7.1.0` fixadas por SHA; `go-version-file: go.mod`; `permissions: contents: read`; `persist-credentials: false` | Um job basta para 3 pacotes com teste (~1s cada). SHA evita que uma tag reescrita mude o que roda. Descartado: matriz de versões de Go/SO (custo sem ganho para um binário único) e golangci-lint (dependência nova; propor depois se houver demanda) |
| 2026-11-20 | Verificação da Fase 0 feita com Postgres descartável próprio (`tracelog-phase0-pg`, 127.0.0.1:55432), removido ao final | Confirmar hipóteses executando sem tocar no banco do usuário; a 5432 local está ocupada por outro projeto (`ecommerce-db-1`) |

## Medições (Fase 0)

Ambiente: Linux 6.12 amd64, Go 1.24.4 (`GOTOOLCHAIN=auto`), Docker 29.10, Compose v5.8, imagem `postgres:16-alpine`.

| Comando | Resultado |
|---------|-----------|
| `go build ./...` | ok |
| `go vet ./...` | ok, sem avisos |
| `go test ./...` | ok: 16 testes em 3 pacotes (`services` 12, `config` 3, `repository` 1); `http`, `cmd`, `domain` sem testes |
| `go test -race -count=1 ./...` | ok |
| `gofmt -l .` | vazio |
| Migrations 000001 e 000002 aplicadas duas vezes | ok, só NOTICEs de "already exists" |

Limitação: as provas HTTP foram feitas com `curl` contra o binário real, não com testes automatizados (esses vêm na Fase 2).

## Medições (Fase 1)

| Verificação | Resultado |
|-------------|-----------|
| Passo gofmt do CI rodado localmente no repo | exit 0 |
| Mesmo passo numa cópia com `bad.go` mal formatado | lista `bad.go`, exit 1 |
| `go vet ./...`, `go test -race -count=1 ./...`, `go build -o bin/tracelog .` | ok |
| Com Go 1.26.11: `gofmt -l`, `go vet`, `go test -race -count=1`, `go build`; `go mod tidy` não alterou nada além da diretiva | ok |
| YAML do workflow | parseia (PyYAML); actionlint não instalado, não validado com ele |
| `LICENSE` | idêntico byte a byte a `/usr/share/common-licenses/GPL-3` |

Limitação: o workflow só roda de verdade no GitHub depois do push.

## Medições (Fase 7)

Verificação a partir de um clone limpo de `release/v0.1.0` (`228ecd7`), sem arquivos não versionados:

| Verificação | Resultado |
|-------------|-----------|
| `gofmt -l .` | vazio |
| `go vet ./...` | ok |
| `go build .` | ok |
| `go test -race -count=1 ./...` com `TRACELOG_TEST_DATABASE_URL` (PostgreSQL 16.16 descartável) | 64 funções `Test*` passaram, 0 falhas, 0 skips; 6 pacotes ok |
| `docker build --no-cache` (imagens base em cache) | 26 s, imagem de 31,3 MB |
| `cp .env.example .env` (só `DB_PORT` trocado, porque a 5432 local é de outro projeto) + `make demo` | exit 0 em 21 s; 429 linhas importadas, 201/200/415, pico de 2026-03-10 com 25 error + 1 fatal |

Sobra: volumes de teste `clean-v010_tracelog_postgres_data`, `tracelog-demo_tracelog_postgres_data`, `tracelog-demo2_tracelog_postgres_data` e `tracelog-phase3_tracelog_postgres_data` (só dados sintéticos). Nenhum container ficou.

## Auditoria doc × código (Fase 6)

| Documento | Afirmação (exemplos) | Realidade verificada | O que mudou |
|-----------|----------------------|----------------------|-------------|
| `docs/http-api.md` | `severity`, `message`, `occurred_at` e IDs obrigatórios; 400 com 5 mensagens; categoria padrão `application`; `occurred_at` não pode ser futuro; consulta por `trace_id` | Tudo opcional e normalizado; só 405/413/415/500; padrão `unknown`; futuro não validado; não há filtro por `trace_id`; faltavam 200/`duplicate`/`/ready` | Apagado; contrato correto no guia §5.1 |
| `docs/cli.md` | Saídas de `events list`, `metrics`, `import`, `db list` e erros | Colunas, totais e mensagens diferentes; `db list` com dados fictícios; faltavam `init`, `migrate`, `--defaults` | Apagado; guia §5.2 com saída real |
| `docs/domain.md` | `Message` obrigatória, `OccurredAt <= ReceivedAt`, severidade validada, `Name` único, canal `file` futuro, `tracelog events create` | Nada disso é aplicado; só `service_key` é único; `file` existe; comando inexistente | Apagado; regras reais no guia §6–§8 |
| `docs/architecture.md` | `lib/pq`, `godotenv`, tabelas no plural, domínio com validação, Docker "futuro", RabbitMQ/Grafana | `pgx/v5`, parser próprio, singular, domínio só com structs, Docker existe, roadmap do AGENTS.md não tem RabbitMQ | Apagado; diagrama fiel no README, arquitetura e desvios no guia §9 |
| `docs/development.md` | Go 1.21, `curl` com `severity`, árvore com arquivos inexistentes, meta de 80% de cobertura | Go 1.26.11; `severity` ignorado; arquivos não existem; não há meta medida | Apagado; desenvolvimento no README e guia §4/§10/§11 |
| `docs/getting-started.md` | Go 1.25+, só caminho interativo | Go 1.26.11; compose faz tudo | Apagado; quickstart no README |
| `README.md` | Passos manuais com IDs do `init`, Go 1.25+, links para os 6 docs | Compose + demo; Go 1.26.11 | Reescrito (inglês), saída real, Mermaid renderizado, "o que não faz" |
| `docs/guia-do-projeto.md` | `.env.example` copiado desatualizado; saída do `serve` escrita à mão; tabela da CLI sem `migrate`; RabbitMQ; "o script reaplica tudo"; seção de divergências | Corrigido contra código e capturas | Reescrito; +modelo de dados, +desvios (H18), +limitações (H11, fallback, corte UTF-8) |
| `AGENTS.md` | Seção 8 apontava divergências dos docs antigos | Docs antigos removidos | Seção 8 diz onde fica cada coisa e exige saída real |

Verificação: 0 links relativos quebrados em `*.md`; Mermaid do README renderizado com mermaid 10.9.1 (12 nós, sem erro); comandos citados executados contra o stack (`up --wait`, `curl` do README → 201, `curl -d` sem cabeçalho → 415, UUID malformado → erro SQLSTATE 22P02 e exit 1, `docker compose run --rm api db list sources`). Tamanho: README 177 linhas, guia 452 linhas (`wc -l`); `docs/` passou de 3.592 para 452 linhas de Markdown (mais specs, `demo-output.txt` e `demo.tape`).

## Medições (Fase 4)

Máquina: Intel i5-1135G7 (8 threads), Linux 6.12, PostgreSQL `16.16-alpine` em Docker (volume local), binário no host. Banco descartável `tracelog-p4-pg` (127.0.0.1:55435), removido ao final.

| Medição | Resultado |
|---------|-----------|
| Dataset versionado | 434 linhas, 60 KB (`checkout-api` 212, `auth-service` 173, `legacy-gateway` 49) |
| Import do dataset (banco vazio) | 429 gravadas, 5 em branco puladas, 378 normalized, 20 partial, 31 uncategorizable |
| Reimport do dataset | 0 gravadas, 429 duplicatas |
| `metrics errors --from 2026-03-01 --to 2026-03-14` | 11 dias; pico em 2026-03-10 com 25 error + 1 fatal |
| Volume gerado `-scale 200` | 80.271 linhas, 8,8 MB |
| Import 80.266 linhas, `synchronous_commit=on` (antes) | 334,0 s (~240 linhas/s) |
| Mesmo import, `synchronous_commit=off` só na conexão (depois) | 34,4 s (~2.330 linhas/s) |
| Reimport das 80.266 (todas duplicatas) | 35,0 s |
| Diagnóstico: 8.402 linhas com `ALTER DATABASE ... synchronous_commit` on/off | 34,2 s / 3,5 s |
| Tabela após 80.266 eventos | 97 MB (`pg_total_relation_size`) |
| `scripts/demo.sh` em projeto compose novo (imagens base em cache) | exit 0 em 32 s; segunda execução sem gravar nada |

Limitações: uma máquina, uma execução por medição (sem média nem desvio), Docker local com disco SSD; o ganho do `synchronous_commit=off` depende do custo de fsync do disco. O reimport (35 s) não foi otimizado: cada duplicata faz o INSERT com conflito e um SELECT do existente.

## Medições (Fase 3)

Projeto compose separado (`-p tracelog-phase3`, `--env-file` em scratch com `DB_PORT=55434` e `HTTP_PORT=18082`, porque a 5432 local é de outro projeto). Nenhum `.env` do repositório foi criado ou lido.

| Verificação (spec 0003 §7) | Resultado medido |
|-----------------------------|------------------|
| Build da imagem (cache frio de camadas do projeto, imagens base já baixadas) | ~41s; `tracelog:local` = 31,3 MB |
| 1. `up -d` com volume vazio | `migrate` e `init` exit 0; `api` healthy em 32s (primeira vez) |
| 2. `up -d` de novo | `api` healthy em 1s; `migrate`: "database is up to date (3 migrations)"; `init`: mesmo application ID |
| 3. `tracelog migrate` duas vezes (CLI no host) | aplica 3, depois nada |
| 4. Banco migrado pelos `.sql` crus | teste de integração `MigrateAdoptsDatabaseFromOldScript` passa |
| 5. `init --defaults` | não lê stdin (teste com reader que falha); IDs iguais em duas execuções (CLI e teste de integração) |
| 6. Usuário do processo | `uid=10001(tracelog)` (`docker exec ... id` e `docker top`) |
| 7. `down -v` + `up -d` (confirmado pelo autor) | volume removido; tudo recriado, `api` healthy em 13s, 0 eventos |
| `docker compose stop api` | exit 0, log `[serve] Shutdown complete` |
| Portas | `ss -ltn`: `127.0.0.1:18082` e `127.0.0.1:55434` |
| Compose sem variáveis | erro de interpolação `defina DB_NAME no .env (copie o .env.example)` |
| `make lint`, `make test`, `make build`, `make migrate` | ok; `make up` sem `.env` falha com instrução |

Limitações: a primeira subida (32s) é dominada pelos intervalos dos healthchecks (5s no Postgres, 10s na API), não pelo trabalho; não medi em máquina limpa sem as imagens base em cache.

Sobra dos testes: ver "Medições (Fase 7)".

## Medições (Fase 2)

Teste de fumaça com o binário real contra `postgres:16-alpine` descartável (`tracelog-it-pg`, 127.0.0.1:55433, removido ao final), migrations 000001–000003 aplicadas e `init < /dev/null`.

| Critério (spec 0002 §7) | Resultado medido |
|--------------------------|------------------|
| 1. SIGTERM com upload lento em andamento (~900 KB a 300 KB/s) | requisição terminou com 201; conexão nova recusada logo após o sinal; processo saiu e imprimiu `Shutdown complete` |
| 2. Corpo de 2 MB | 413 `request body exceeds 1048576 bytes`, nada gravado |
| 3. `text/plain` / sem Content-Type | 415 / 201 |
| 4. Banco parado durante `POST /events` | 500 `{"error":"internal error"}`; detalhe (`dial tcp ... connection refused`) só no log |
| 5. Mesmo `external_id` duas vezes | 201 e 200 `duplicate:true`, mesmo `id`; apps diferentes coberto por teste de integração |
| 6. `/ready` com banco parado / de volta | 503 / 200; `/health` 200 nos dois casos; `DELETE /health` → 405 com `Allow: GET, HEAD` |
| 7. `payload.level=fatal` via HTTP | gravado `info` |
| `"terror alert dismissed"` | gravado `info` (antes: `error`) |
| `external_id` de 300 caracteres | 201, gravado com 255 |
| JSON inválido | 201 com `raw_request_body`; eventos válidos ficam sem a cópia |
| `metrics errors --from 2026-03-03 --to 2026-03-03` | mostra o dia 3 com ERRORS 1, FATAL 1 (antes o dia 3 ficava fora e fatal não contava) |
| `go test -race -count=1 ./...` | ok em 5 pacotes; integração: 4 testes passam com banco, 4 `SKIP` sem |
| Teste de dedupe sem a migration 000003 | falha com SQLSTATE 42P10, confirmando a dependência da migration |
| Tabela de severidade contra o código antigo | falhavam exatamente os 4 casos de falso positivo por substring |

Limitações: nenhum teste de carga; os timeouts foram verificados por configuração e pelo teste de shutdown, não por ataque *slowloris* real.

## Achados verificados

Ver o relatório da Fase 0 na conversa; resumo por hipótese:

| # | Veredito | Evidência |
|---|----------|-----------|
| H1 | confirmada | `server.go:34` `http.ListenAndServe`, sem timeouts; SIGTERM durante upload matou a conexão em andamento |
| H2 | confirmada | `event_handler.go:30` `io.ReadAll`; `:60` `context.Background()`; corpo de 20 MB aceito com 201 |
| H3 | confirmada | `event_handler.go:62`; resposta real expõe `duplicate key value violates unique constraint ... (SQLSTATE 23505)` |
| H4 | confirmada | `000001:55` UNIQUE global; 23505 não tratado (`technical_event_repository.go:114`) → 500 |
| H5 | confirmada + duplicação | JSON inválido/vazio/text-plain → 201; corpo bruto copiado em `_ingestion.raw_request_body` sempre que há qualquer aviso (quase sempre), duplicando `message` e `payload` |
| H6 | confirmada | `server.go:37`; `DELETE /health` → 200 |
| H7 | confirmada | 6 de 8 mensagens de teste viram `error` por substring ("0 errors found", "terror alert dismissed", "report generated without failure") |
| H8 | confirmada | 2 eventos `fatal` fora da contagem; dias sem erro omitidos |
| H9 | confirmada | `register_technical_event_usecase.go:389-398` |
| H10 | confirmada | senha `a b` falha a autenticação (`config.go:47-57`) |
| H11 | confirmada | `DB_NAME=tracelog # comentario` corrompe o DSN e o usuário vira o do SO |
| H12 | confirmada | `scripts/run-migrations.sh:23-29` |
| H13 | confirmada | `000001:6`; PG16 tem `gen_random_uuid()` nativo |
| H14 | confirmada (impacto baixo) | `postgres.go:18` sem limites |
| H15 | confirmada | `cmd/seed.go:10` |
| H16 | parcial | `init < /dev/null` já funciona com padrões e é idempotente (mesmos IDs); falta só flags/clareza |
| H17 | confirmada | reimportar o mesmo arquivo dobrou as linhas (6 → 12) |
| H18 | confirmada + agravante | `cmd/db.go`; além disso `cmd.Println` do Cobra escreve em **stderr**: tabela vai para stdout e o resumo para stderr |
| H19 | confirmada | sem testes de handler/repositório; `docs/development.md:189` cita testes de integração inexistentes |
| H20 | confirmada, com 1 refutação | `/api/v1/auth/login` em `docs/domain.md:175` é só um exemplo de `description` de EventSource, não resquício de rota |

Achados novos (não estavam nas hipóteses): ver "Achados fora de escopo" e o relatório.

## Achados fora de escopo (para fases futuras)

- `payload.level` enviado pelo cliente HTTP define a severidade (`deriveSeverity` checa `payload.level` primeiro): `{"payload":{"level":"fatal"}}` gravou `fatal`. Contradiz a regra "severity não é confiável vinda do cliente". → Fase 2.
- UUIDs v7 (e v6/v8) são tratados como inválidos (`uuidLikePattern` exige versão 1-5) e caem silenciosamente no fallback. → Fase 2.
- FK inexistente com UUID válido também cai no fallback com 201 (comportamento documentado, mas o cliente não fica sabendo).
- `--to YYYY-MM-DD` vira 00:00 UTC com `<=`: o próprio dia fica fora (evento de 2026-03-03T10:00 sumiu com `--to 2026-03-03`). → Fase 2 ou 6.
- Categorizador do import: `"unauthorized"` contém `"auth"`, então cai em `audit` antes de `security`; o ramo de segurança para "unauthorized" é inalcançável. → Fase 4 (dataset de segurança).
- Linhas em branco no import viram eventos "event received without message". → Fase 4.
- `external_id` > 255 caracteres → 500 com mensagem SQL.
- `events list` trunca a mensagem por bytes (`message[:97]`), podendo cortar UTF-8 no meio.
- `.gitignore` ignora `*.log`: arquivos em `examples/logs/*.log` não seriam commitados. → Fase 4.
- Na máquina local a porta 5432 já está publicada por `ecommerce-db-1` (0.0.0.0). Com o `.env.example` atual (`DB_HOST=localhost`, `DB_PORT=5432`), o compose do TraceLog não sobe e a CLI conectaria no Postgres de outro projeto. → Fase 3.
- `go fix -diff` (Go 1.26) sugere modernizações (`strings.CutPrefix`, `maps.Copy`). Não aplicadas: refatoração sem mudança de comportamento; avaliar num commit `refactor:` à parte.
- O guia diz "Go 1.24.4+"; agora o correto é 1.26.11. → Fase 6.
- ~~`serve` imprime "listening" antes de abrir a porta.~~ Corrigido na fase 2 (listen antes da mensagem).
- H11 (comentário inline no `.env` entra no valor) continua aberto: o parser caseiro não foi alterado. → Fase 6 (documentar) ou commit próprio.
- `docs/demo.tape` usa `localhost:8080` no `curl`; com outro `HTTP_PORT` no `.env`, ajustar o roteiro.
- A saída de `docs/demo-output.txt` tem uma linha em branco extra depois de cada JSON (o `-w` do curl começa com `\n`). Cosmético.
- `events list` e `init` ainda escrevem parte da saída em stderr (`cmd.Println`); na demo isso não aparece porque a saída é capturada com `2>&1`.
- `curl -d` sem `-H 'Content-Type: application/json'` agora recebe 415: README e docs precisam mostrar sempre o cabeçalho. → Fase 6.
- `cmd.Printf`/`cmd.Println` (stderr) continuam em `init`, `serve`, `events list`, `import logs`, `db list`; só `metrics errors` foi corrigido. → Fase 4/6 ou commit próprio.
- `docs/http-api.md`, `docs/cli.md`, `docs/domain.md` não foram atualizados na fase 2 (só o guia, como pede o AGENTS.md). → Fase 6.
- `LoadBasicSetup` usa sempre o registro mais antigo de cada tabela como fallback, mesmo que `init` seja rodado com outra `service_key`.

## Próximo passo

Trilha 1: nada pendente além do opcional (gravar o GIF com `docs/demo.tape`; a Fase 5, medição de banco, não foi pedida). PR, merge, tag `v0.1.0` e CI verde foram concluídos e verificados na Fase 0 da Trilha 2. O próximo passo do projeto está na Trilha 2, abaixo.

---

# Trilha 2: API de leitura e dashboard

Objetivo: API JSON de leitura e dashboard web (React + TypeScript em `web/`) no mesmo repositório. Antecipa os itens 4 (API JSON e UI web) e 6 (dashboards) do roadmap do `AGENTS.md` por decisão do autor (portfólio). As specs das fases 1 e 3 são a aprovação exigida pelo `AGENTS.md` para sair do escopo do MVP.

## Fase atual (Trilha 2)

**Fase 3 concluída:** spec 0006 aprovada pelo autor (2026-11-23) em `feat/web-dashboard`. Esperando o "continuar" para a Fase 4 (scaffold de `web/` e CI).

## Checklist (Trilha 2)

| Fase | Branch | Status |
|------|--------|--------|
| 0. Orientação e portão | `feat/read-api` | concluída; P0–P4 satisfeitos |
| 1. Spec 0005 (API de leitura) | `feat/read-api` | concluída; aprovada em 2026-11-23 |
| 2. Implementação da API de leitura | `feat/read-api` | concluída; PR #5 mesclado (`6902f5e`), CI verde no `main` |
| 3. Spec 0006 (interface web) | `feat/web-dashboard` | concluída; aprovada em 2026-11-23 |
| 4. Scaffold de `web/` e CI | `feat/web-dashboard` | pendente |
| 5. Telas (Eventos, Detalhe, Visão geral) | `feat/web-dashboard` | pendente |
| 6. Integração e entrega | `docs/dashboard` | pendente |
| 7. Documentação e release | `docs/dashboard` | pendente |

## Decisões pendentes (Trilha 2)

1. ~~OK para refatorar `cmd/db.go`.~~ Dado pelo autor; feito em commit separado (`refactor:`).
2. ~~Índice `(severity, occurred_at DESC, id DESC)`.~~ Aprovado pelo autor; migration 000004.
3. Dataset com uma aplicação só (spec 0005, questão 2): decidir antes da Fase 6.
4. ~~Spec 0006, seção 8~~ (aprovada com todas as recomendações): (a) servir (embutido com `go:embed`); (b) caminho (`/ui/`); (c) gráfico (SVG próprio); (d) roteamento e dados (React Router + TanStack Query); (e) DOM dos testes (happy-dom); (f) Node (`24.13.0` exato); (g) lint (ESLint sem Prettier); (h) fuso (UTC); (i) idioma da UI (inglês); (j) URL e paginação (cursor na query, anterior pelo histórico); (k) fixtures compartilhadas (sim, alias `@fixtures`).

## Log de decisões (Trilha 2)

| Data | Decisão | Porquê |
|------|---------|--------|
| 2026-11-23 | Antecipar "API JSON e UI web" (roadmap item 4) e dashboards (item 6) para antes do núcleo de investigação (item 1) e do SQLite (item 2) | Decisão do autor (portfólio). Custo: a leitura nasce sobre a camada PostgreSQL específica; quando o SQLite vier, as consultas novas também terão de ser reescritas. Mitigação: leitura atrás de ports, SQL só nos adapters |
| 2026-11-23 | Spec 0005 aprovada com todas as recomendações: `/api/v1` (sem espelhar `POST /events`); keyset `(occurred_at, id)` com cursor opaco não assinado; dias vazios preenchidos no Go com `from`/`to` obrigatórios e teto de 366 dias; lista sem `payload`/`raw_line`/`parse_error`, detalhe completo; `HTTP_HOST` padrão `127.0.0.1` (compose `0.0.0.0`), sem CORS; `limit` 1–200 e `400` estrito com `code`/`field`; `data-range` com 14 dias até o erro mais recente; índice só com medição; três rotas de referência; sem severidade múltipla; cursor reaproveitável entre filtros | Decisão do autor; justificativas e alternativas descartadas na spec 0005 §8 |
| 2026-11-23 | Spec 0006 aprovada com todas as recomendações: UI embutida com `go:embed` sob `/ui/` (`/` redireciona), build do Vite no módulo Go raiz e `web/go.mod` marcador; gráfico em SVG próprio; React Router + TanStack Query; happy-dom; Node `24.13.0` exato; ESLint sem Prettier; horários em UTC; UI em inglês; filtros e cursor na URL com "anterior" pelo histórico; fixtures de `testdata/api` por alias `@fixtures` | Decisão do autor; justificativas e alternativas descartadas na spec 0006 §8 |
| 2026-11-23 | TypeScript 6.0.3 em vez de 7.0.2; `@testing-library/dom` 10.4.2 acrescentado (spec 0006 §5.4 atualizada) | Decisão do autor. Os peers do registro npm mostraram que o `typescript-eslint` 8.74 exige `typescript <6.1.0` e que o `@testing-library/react` 16 exige `@testing-library/dom ^10`. Descartados: TS 7 sem `typescript-eslint` (sem lint de TSX, perde a regra contra `dangerouslySetInnerHTML`) e `--legacy-peer-deps` |
| 2026-11-23 | Spec 0006 e fase 4 no mesmo branch `feat/web-dashboard` | Como spec 0005 + fase 2 em `feat/read-api`: a spec é aprovada na conversa, sem PR próprio; o PR leva spec e scaffold juntos |
| 2026-11-23 | Índice `idx_technical_event_severity_occurred_at` por migration 000004 | Decisão do autor (eu havia recomendado esperar: sem ele, toda consulta de tela já ficava abaixo de 45 ms). Ganho medido na lista filtrada e na contagem da Visão geral; custo ~8 µs por linha e 3,9 MB a cada 80 mil eventos. O índice só de `severity` ficou redundante e não foi removido (pede confirmação) |
| 2026-11-23 | Novo caso de uso `ListEventsPageUseCase` em vez de mudar `ListTechnicalEventsUseCase` | A CLI não tem teto de `limit` e a spec 0005 exclui mudanças na CLI; os dois compartilham o port. Descartado: um caso de uso com modo "estrito" (flag que muda a validação) |
| 2026-11-23 | `Limit *int` no input da página | Distingue ausente (padrão 50) de `limit=0` (400) sem o adapter decidir o padrão |
| 2026-11-23 | Parsing de query string e cursor no adapter HTTP; faixas, `from <= to`, UUID e período máximo nos casos de uso | Codificação é detalhe de transporte; regra vale para qualquer adapter (MCP no futuro) |
| 2026-11-23 | Parser de `from`/`to` duplicado no adapter HTTP (o da CLI fica em `cmd/events.go`) | Unificar seria refatoração da CLI, fora do escopo da fase; registrado em achados |
| 2026-11-23 | `data-range` com três subconsultas | `MIN`/`MAX` isolados usam o índice de `occurred_at` (Index Only Scan); um agregado só com `FILTER` faria Seq Scan |
| 2026-11-23 | `ENV HTTP_HOST=0.0.0.0` também no `Dockerfile`, além do compose | Sem isso, `docker run -p` fora do compose ficaria inalcançável; registrado na spec 0005 §4.7 |
| 2026-11-23 | Fixtures golden comparadas byte a byte depois de `json.Indent`, regravadas com `-update` | Legíveis para o front e estritas em campo e ordem; mudança de contrato exige regravar conscientemente |
| 2026-11-23 | README: só as frases que ficaram falsas e o diagrama | Deixar "one HTTP endpoint" e "`db list` direto" após o merge seria documentação falsa; a seção do dashboard é da fase 7 |
| 2026-11-23 | Verificação da Fase 0 em clone limpo (`git clone` local em scratch) e ambiente descartável: Postgres `tracelog-t2p0-pg` em `127.0.0.1:55440`, demo com projeto compose `tracelog-t2p0` (`DB_PORT=55441`, `HTTP_PORT=18090`, `--env-file` fora do repositório) | A 5432 local é de outro projeto; nada do `.env` do repositório foi lido ou criado |
| 2026-11-23 | CI do `main` consultado pela API pública do GitHub (`/commits/<sha>/check-runs`), porque o `gh` não está instalado | Regra: não presumir CI verde |

## Medições (Trilha 2, Fase 0)

Máquina: a mesma da trilha 1 (i5-1135G7, Linux 6.12), Go 1.26.11, PostgreSQL `16.16-alpine` em Docker. Uma execução por medição, cache quente (`shared hit`), sem média.

| Verificação | Resultado |
|-------------|-----------|
| P0 `git rev-parse v0.1.0^{commit}` | `492f362` (merge do PR #4, HEAD do `main`); tag anotada; `CHANGELOG.md` nesse commit tem `## [0.1.0] - 2026-11-21` |
| P1 `main` x `origin/main` | iguais (`492f362`), working tree limpo; specs 0001–0004, migrations 000001–000003, `Dockerfile`, `Makefile`, `examples/logs`, `tools/loggen`, `scripts/demo.sh` presentes |
| P2 CI em `492f362` | `format, vet, test, build`: success; `docker build`: success (2026-11-23T20:11Z) |
| P3 `make lint`, `make test` (com `TRACELOG_TEST_DATABASE_URL`), `make build` em clone limpo | exit 0; 64 `Test*` de topo passaram, 0 falhas, 0 skips, 6 pacotes ok; binário 19 MB |
| P4 `make demo` (projeto compose novo, imagens base em cache) | exit 0 em 20 s; saída igual a `docs/demo-output.txt` normalizando UUIDs, horários de recebimento, porta e uptime (única diferença: o eco `./scripts/demo.sh` do `make`) |
| Dataset versionado importado | 429 eventos, **1** aplicação, 1 ambiente; 14 dias com dados em março (11 com error/fatal); **51 eventos (12%) com `occurred_at` = hora do import** (2026-11-23), todos `partial`/`uncategorizable` (50 info, 1 warn) |
| `loggen -scale 200` importado | 80.266 eventos em 38,4 s; tabela 97 MB; info 60.314, warn 14.906, error 5.045, **fatal 1** (o fatal não escala); 8.409 eventos com `occurred_at` = hora do import; 4.638 timestamps repetidos cobrindo 16.129 linhas |
| Lista sem filtro (`ORDER BY occurred_at DESC LIMIT 50`) | Index Scan `idx_technical_event_occurred_at`, 0,04 ms, 7 buffers |
| Lista `severity='error'` | Index Scan em `occurred_at` + Filter, 26.913 linhas descartadas, 6,1 ms, 11.041 buffers |
| Lista `severity='fatal'` + período | Index Scan `idx_technical_event_severity` + Sort, 0,05 ms |
| Frequência de erros (março) | Index Scan `idx_technical_event_severity` (5.046 linhas) + Sort + GroupAggregate, 2,9 ms |
| Keyset `(occurred_at, id) <` + `ORDER BY occurred_at DESC, id DESC LIMIT 51` | Index Scan em `occurred_at` + **Incremental Sort**, 0,18 ms: o índice de coluna única já atende o desempate |
| `OFFSET 40000` com a mesma ordenação | Seq Scan + Sort em disco (2,7 MB), 43 ms |
| Contagem por severidade no período | Seq Scan, 30 ms |

Limitações: cache quente, uma execução, um volume; os planos com filtro de aplicação/ambiente não foram medidos porque o dataset tem uma aplicação e um ambiente só. A medição formal com proposta (ou não) de índice é da Fase 2.

Sobra: volume `tracelog-t2p0_tracelog_postgres_data` (dados sintéticos), somado aos da trilha 1. Nenhum container ficou rodando.

## Medições (Trilha 2, Fase 2)

Mesma máquina. PostgreSQL `16.16-alpine` descartável (`tracelog-t2p2-pg`, 127.0.0.1:55442), `loggen -scale 200` importado (80.266 eventos, 97 MB, uma aplicação e um ambiente), `VACUUM ANALYZE`. `EXPLAIN (ANALYZE, BUFFERS)` das consultas exatas do repositório, três execuções, cache quente.

| Consulta | Plano (sem índice novo) | Tempo (3 execuções, ms) | Com o candidato |
|----------|-------------------------|-------------------------|-----------------|
| L1 lista, 1ª página (`LIMIT 51`) | Index Scan `occurred_at` + Incremental Sort | 0,22 / 0,48 / 0,25 | 0,18–0,41 |
| L2 página profunda por cursor (posição 40.000) | idem, 31 buffers | 0,20 / 0,24 / 0,18 | 0,16–0,36 |
| L3 `severity=error` | Index Scan `occurred_at`, 26.913 linhas descartadas, 11.027 buffers | 11,5 / 12,9 / 11,3 | **0,19–0,24** |
| L4 `severity=error` + cursor | idem, 8.401 descartadas | 3,1 / 4,1 / 3,3 | 0,06–0,48 |
| L5 `severity=fatal` | Index Scan `severity` | 0,05–0,08 | 0,07–0,20 |
| L6 aplicação + período padrão | Index Scan `occurred_at` | 0,12–0,18 | 0,11–0,16 |
| F1 frequência de erros (14 dias) | Index Scan `severity` (5.046 linhas) + Sort | 3,3–5,6 | 2,3–2,4 |
| C1 contagem por severidade (14 dias) | **Seq Scan** (10.087 buffers) | 31,4 / 36,8 / 42,8 | **11,7–13,6** (Index Only Scan) |
| D1 data-range | 2 Index Only Scan + Index Scan com filtro | 2,1–2,9 | 3,3–5,5 |
| I1 detalhe por id | Index Scan `technical_event_pkey` | 0,03–0,07 | 0,04–0,15 |

HTTP ponta a ponta (`curl -w %{time_total}`, 5 execuções, servidor no host): lista 3,3–5,9 ms; `severity=error` 15,5–20,9 ms; `limit=200` 3,3–4,4 ms (115 KB); frequência 3,8–9,7 ms; contagem 29,8–51,1 ms; data-range 2,4–6,4 ms; aplicações 0,5–1,7 ms.

Custo do índice candidato `(severity, occurred_at DESC, id DESC)`: 3,9 MB para 80.266 linhas. `INSERT INTO clone SELECT * FROM technical_event` (80.266 linhas, cópia com os mesmos índices) sem/com o candidato: 4.022/5.050, 6.170/6.686, 5.845/6.262 ms, ou seja, +0,4 a +1,0 s (~8 µs por linha). No import real isso é 1–2% de 34–69 s. Imports reais de `-scale 20` (8.064 linhas) sem/com: 5,3/8,1, 5,3/6,5, 8,7/7,1 s, ruído maior que o efeito.

Com a migration 000004 aplicada pelo `tracelog migrate` (banco novo, mesmo volume importado depois): `EXPLAIN (ANALYZE, BUFFERS)`, três execuções, em ms: L1 0,17–0,23; L2 0,18–0,32; L3 **0,17–0,39**; L4 0,07–0,47; L5 0,06–0,23; L6 0,12–0,30; F1 2,8–4,6; C1 **14,2–26,1** (Index Only Scan no índice novo); D1 2,8–4,5; I1 0,05–0,06. O planejador usa o índice novo só em L3, L4, F1 e C1; os demais planos não mudaram. Import das 80.266 linhas com o índice: 65,8 s (68,8 s sem, hoje: diferença dentro do ruído). `CREATE INDEX` sobre 80.266 linhas: 190 ms. Limitação: o Index Only Scan da contagem depende do *visibility map*; logo depois de um import grande, antes do autovacuum, ele visita mais páginas da tabela.

Outras verificações: integração (13 testes, inclusive 9 eventos com 7 timestamps iguais em páginas de 3) com `TRACELOG_TEST_DATABASE_URL`; mutação `occurred_at < cursor` (keyset sem desempate) faz o teste de paginação falhar (`pages = 2, want 3`); `ss -ltn`: `127.0.0.1:18091` sem `HTTP_HOST` e `*:18091` com `0.0.0.0`; `db list` antes/depois da refatoração com `cmp` idêntico em stdout e stderr; `scripts/demo.sh` em projeto compose novo (`tracelog-t2p2`): exit 0 em 46 s, API de leitura respondendo na porta publicada, portas só em `127.0.0.1`, imagem 31,5 MB; exemplos `curl` do guia §5.1.1 capturados contra `examples/logs` importado.

Limitações: uma máquina, cache quente, volume com uma aplicação e um ambiente (filtros por eles não discriminam); o import de hoje levou 68,8 s contra 38,4 s na Fase 0 com o mesmo binário de import e o mesmo volume, o que indica variação da máquina (Docker e disco compartilhados), não regressão: o caminho de escrita não mudou. O diagrama Mermaid do README não foi validado por renderização (sem `mmdc` local).

## Medições (Trilha 2, Fase 3)

| Verificação | Resultado |
|-------------|-----------|
| CI no merge do PR #5 (`6902f5e`) | `format, vet, test, build` e `docker build`: success; árvore idêntica a `9cd8458` (head do PR, também verde) |
| `go vet ./...` num módulo com `web/node_modules/pkg/bad.go` | falha (`expected declaration`): o Go percorre `node_modules` |
| Mesmo teste com `web/go.mod` mínimo | exit 0; `go list ./...` não lista `web/` |
| `flatted` (dependência do ESLint via `flat-cache`) no unpkg | publica `golang/pkg/flatted/flatted.go` |
| `jsdom` 30.x e 31.x no registro npm | `engines.node`: `^22.22.2 \|\| ^24.15.0 \|\| >=26.0.0`; Node local: 24.13.0 |
| `happy-dom` 21.0.4 | `engines.node`: `>=20.0.0` |
| Recharts 3.11.1 | 11 dependências (inclui `@reduxjs/toolkit`, `react-redux`, `immer`, `victory-vendor`), 8,5 MB publicados |

Método: metadados do registro npm (`/<pacote>/latest`, `unpackedSize`, `engines`, `dependencies`) e listagem do unpkg, sem instalar nada. `unpackedSize` não é tamanho de bundle; o bundle será medido no build (fase 4).

## Achados fora de escopo (Trilha 2)

- O mux devolve `404 page not found` em `text/plain` para rotas desconhecidas (padrão do `net/http`), não JSON. A API de leitura vai criar o primeiro 404 de verdade; decidir na spec 0005 se rotas desconhecidas também respondem JSON.
- `ListTechnicalEventsUseCase` e `GetErrorFrequencyUseCase` devolvem erros de validação como `fmt.Errorf` sem tipo: o adapter HTTP não tem como distinguir 400 de 500 (relevante para a spec 0005).
- `tools/loggen -scale N` não escala o evento `fatal` (1 em 80 mil); o gráfico de fatais fica quase vazio com o volume grande.
- Eventos sem timestamp utilizável recebem `occurred_at` = hora do import (comportamento documentado no guia §5.3), o que espalha o dataset de março até o dia do import.
- `POST /events` devolve `received_at`/`created_at` com o offset do fuso do processo (ex.: `-03:00` com `serve` no host); a leitura normaliza para UTC. Os exemplos do guia mostram `Z` porque a demo roda em container UTC.
- `docs/demo-output.txt` depende da ordem em que o `docker compose run` mistura stdout e stderr: o `Total: N event(s).` do `events list` (stderr) saiu antes da tabela na captura versionada e depois dela na execução da fase 2. Corrigir de vez exige a saída da CLI toda em stdout (pendência do guia §13).
- Parser de `from`/`to` duplicado entre `cmd/events.go` e `internal/adapters/http/read_handler.go` (mesma regra); unificar num commit `refactor:` próprio.
- A mensagem do `serve` ("listening on port N") não mostra o `HTTP_HOST`; mantida para não mudar a saída da CLI sem spec.
- `idx_technical_event_severity` (000001) ficou redundante com o índice composto da 000004 (mesma coluna à esquerda). Removê-lo economiza escrita, mas é `DROP INDEX` numa migration nova e pede confirmação (AGENTS.md §12).

## Próximo passo (Trilha 2)

Autor: responder "continuar". Depois: Fase 4 em `feat/web-dashboard`: `web/` (Vite, React, TS strict, Tailwind, React Router, TanStack Query), `web/go.mod`, cliente HTTP e tipos da spec 0005, lint/typecheck/test/build, alvos `web-*` no `Makefile`, job `web` no CI e página de prova de vida via proxy do Vite.
