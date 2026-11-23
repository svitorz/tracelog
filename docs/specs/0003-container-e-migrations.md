# Spec 0003: Container, migrations versionadas e setup não interativo

- **Status:** Aprovada (decisões confirmadas pelo autor em 2026-11-20)
- **Autor:** Vitor Fábio
- **Data:** 2026-11-20
- **Escopo:** comandos `migrate` e `init --defaults`, tabela `schema_migrations`, imagem Docker, `docker-compose.yml`, `Makefile`

## 1. Problema

- `scripts/run-migrations.sh` reaplica todos os `.sql` a cada execução via `docker compose exec psql`. Não funciona com a API em container nem em CI sem Docker, e não há registro do que foi aplicado.
- `tracelog init` é interativo. Funciona com `stdin` fechado por acidente, não por contrato.
- Não existe imagem da aplicação: subir o TraceLog exige Go instalado e quatro comandos manuais.
- O compose publica o PostgreSQL em todas as interfaces (`0.0.0.0`).

## 2. Objetivos

`cp .env.example .env` seguido de `docker compose up` sobe PostgreSQL, aplica as migrations, grava o setup padrão e inicia a API, sem outros passos.

## 3. Não objetivos

- Migrations *down* (rollback). Correções são feitas com uma migration nova, como hoje.
- TLS, autenticação, secrets manager.
- Imagem publicada em registry.

## 4. Comportamento e contrato

### 4.1 `tracelog migrate`

- As migrations de `migrations/*.sql` são embutidas no binário (`go:embed`).
- Tabela de controle `schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`, criada pelo próprio comando.
- Os arquivos rodam em ordem lexicográfica; cada um roda **uma vez** e é registrado em seguida. `version` é o nome do arquivo sem `.sql`.
- Um *advisory lock* do PostgreSQL impede que duas execuções simultâneas apliquem a mesma migration.
- Banco já migrado pelo script antigo (sem `schema_migrations`): todas as migrations rodam de novo, sem erro porque são idempotentes, e passam a ser registradas.
- Saída: uma linha `[migrate] applied <version>` por arquivo aplicado e `[migrate] database is up to date (N migrations)` no fim.
- Cada arquivo controla a própria transação (`BEGIN`/`COMMIT`, como os atuais). Se o processo morrer entre aplicar o arquivo e registrá-lo, a próxima execução reaplica o arquivo; por isso a regra de idempotência do `AGENTS.md` continua valendo.

`scripts/run-migrations.sh` é removido.

### 4.2 `tracelog init --defaults`

Sem a flag, o comportamento interativo não muda. Com `--defaults`, o comando não lê `stdin` e grava os valores padrão (os mesmos dos prompts). Executar de novo não cria registros nem muda IDs (upsert por `environment.code`, `monitored_application.service_key` e `event_source(name, source_type)`).

### 4.3 Imagem

- Build multi-stage: `golang:1.26.11-alpine3.24` → `alpine:3.24`, `CGO_ENABLED=0`, binário em `/usr/local/bin/tracelog`.
- Usuário não-root (UID 10001), `ENTRYPOINT ["tracelog"]`, `CMD ["serve"]`.
- `HEALTHCHECK` com `wget` em `/ready`.

### 4.4 Compose

| Serviço | Imagem | Papel | Depende de |
|---------|--------|-------|------------|
| `postgres` | `postgres:16.16-alpine` | Banco, volume `tracelog_postgres_data` | — |
| `migrate` | `tracelog:local` | `tracelog migrate`, termina | `postgres` saudável |
| `init` | `tracelog:local` | `tracelog init --defaults`, termina | `migrate` concluído com sucesso |
| `api` | `tracelog:local` | `tracelog serve` | `init` concluído com sucesso |

- Dentro da rede do compose, a aplicação usa `DB_HOST=postgres` e `DB_PORT=5432`, definidos em `environment:`. O `.env` continua servindo para rodar a CLI no host.
- Portas publicadas só em `127.0.0.1`: PostgreSQL em `${DB_PORT}` e API em `${HTTP_PORT}`.
- `DB_NAME`, `DB_USER` e `DB_PASSWORD` são obrigatórios; sem eles o compose falha com mensagem clara.

### 4.5 `Makefile`

Alvos: `test`, `lint` (gofmt + vet), `build`, `up`, `migrate`. O alvo `demo` entra na fase 4, junto com o script de demonstração.

## 5. Segurança do setup local

O compose é para uso local. Ele **não** é seguro para expor na rede:

- a API não tem autenticação: qualquer um que alcance a porta grava eventos;
- não há TLS entre cliente e API nem entre API e banco (`DB_SSLMODE=disable`);
- a senha do banco vem do `.env.example` (`postgres`) e precisa ser trocada;
- não há rate limit.

Publicar as portas só em `127.0.0.1` reduz a exposição, mas não resolve nenhum desses pontos.

## 6. Schema

Nova tabela `schema_migrations`, criada pelo `tracelog migrate`, fora dos arquivos de migration. Nenhuma tabela existente muda.

## 7. Critérios de aceite

1. Com volume vazio, `cp .env.example .env && docker compose up -d` deixa `api` saudável e `migrate`/`init` com código de saída 0.
2. `docker compose up` de novo: `migrate` não aplica nada e `init` mantém os mesmos IDs.
3. `tracelog migrate` duas vezes contra o mesmo banco: a segunda não aplica nada.
4. Banco migrado só pelos `.sql` (sem `schema_migrations`): `tracelog migrate` termina sem erro e registra todas as versões.
5. `init --defaults` não lê `stdin`; duas execuções produzem os mesmos IDs.
6. O processo da API no container não roda como root.
7. `docker compose down -v` seguido de `up` recria tudo do zero.

## 8. Questões em aberto

Nenhuma.

## 9. Decisões registradas

- **`pgcrypto` fica** (decisão do autor, 2026-11-20). `CREATE EXTENSION IF NOT EXISTS pgcrypto` em `000001` é desnecessário no PostgreSQL 16 (`gen_random_uuid()` é nativo desde o 13), mas a migration já commitada não pode ser editada, e removê-la exigiria `DROP EXTENSION` numa migration nova, criando e apagando a extensão em toda instalação. Desde o PostgreSQL 13 a `pgcrypto` é extensão *trusted*: quem tem `CREATE` no banco consegue instalá-la sem superusuário. Nenhum código depende dela.
