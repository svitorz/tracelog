# PROGRESS

Memória de trabalho do preparo do TraceLog para portfólio (v0.1.0). Leia este arquivo antes de qualquer coisa ao retomar uma sessão.

## Fase atual

**Fase 1 (rede de segurança e CI) concluída** no branch `chore/ci-baseline`. Aguardando "continuar" e as decisões da Fase 1 abaixo.

## Checklist

| Fase | Branch | Status |
|------|--------|--------|
| 0. Orientação | `chore/ci-baseline` | concluída |
| 1. Rede de segurança e CI | `chore/ci-baseline` | concluída (CI ainda não rodou no GitHub: depende do push) |
| 2. Robustez da API HTTP | `fix/http-hardening` | pendente |
| 3. Container e migrations | `feat/container` | pendente |
| 4. Dados de exemplo e demo | `feat/sample-data` | pendente |
| 5. Medição de banco (opcional) | — | só se pedido |
| 6. Auditoria da documentação | `docs/audit` | pendente |
| 7. Release v0.1.0 | `release/v0.1.0` | pendente |

## Decisões pendentes (bloqueiam a Fase 1 ou a 2)

1. ~~**Licença.**~~ Decidido: GPL-3.0 (ver log de decisões).
2. ~~**Spec 0001 x prompt.**~~ Decidido: só dedupe na v0.1.0 (ver log de decisões). Texto original: `docs/specs/0001-ingestao-em-lote-idempotente.md` (não commitada, marcada "Aprovada") já decide H4: unicidade `(application_id, external_id)`, duplicata responde `200` com `"duplicate": true`, `external_id` obrigatório no HTTP (400) e cria `POST /events/batch`. Isso conflita com "decida comigo 409 ou devolver existente", com H5 ("não perder evento": sem `external_id` decodificável a requisição viraria 400) e com o escopo enxuto da Fase 2.
3. ~~**Arquivos não commitados.**~~ Decidido: commitados na Fase 1, um commit por arquivo lógico.
4. ~~**Makefile.**~~ Decidido: criar na Fase 3 (ver log de decisões).
5. ~~**Versão do Go.**~~ Decidido: 1.26.11 (ver log de decisões).

## Log de decisões

| Data | Decisão | Porquê |
|------|---------|--------|
| 2026-11-20 | Licença GPL-3.0, não MIT. Na Fase 1, o item "adicionar LICENSE MIT" vira "commitar o `LICENSE` GPL-3.0 e o README já ajustados" | Decisão do autor; o working tree e o `AGENTS.md` já estavam em GPL-3.0 |
| 2026-11-20 | Spec 0001 na v0.1.0: só a parte de duplicidade. Unicidade `(application_id, external_id) WHERE external_id IS NOT NULL` por migration nova; duplicata responde `200` com `"duplicate": true` e o evento existente. Ficam para depois da v0.1.0: `external_id` obrigatório (D5) e `POST /events/batch` (D3). A spec será atualizada na Fase 2 para registrar o recorte | Mantém a Fase 2 pequena e preserva a política de ingestão tolerante (H5): sem `external_id`, o evento continua sendo gravado. Descartado: 409, que obrigaria o cliente a tratar erro num reenvio que deu certo |
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
- `serve` imprime "listening" antes de efetivamente abrir a porta.
- `LoadBasicSetup` usa sempre o registro mais antigo de cada tabela como fallback, mesmo que `init` seja rodado com outra `service_key`.

## Próximo passo

Depois do push, conferir o primeiro run do CI; "continuar" inicia a Fase 2 no branch `fix/http-hardening`.
