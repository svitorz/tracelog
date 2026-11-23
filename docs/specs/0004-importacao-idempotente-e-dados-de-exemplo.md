# Spec 0004: Importação idempotente, relatório de erros e dados de exemplo

- **Status:** Aprovada (decisões confirmadas pelo autor em 2026-11-20)
- **Autor:** Vitor Fábio
- **Data:** 2026-11-20
- **Escopo:** `tracelog import logs`, categorizador de linhas, comando `seed`, `examples/logs/`, `tools/loggen`, `scripts/demo.sh`
- **Relacionada:** [0001](./0001-ingestao-em-lote-idempotente.md) (que deixou a idempotência do import para outra spec)

## 1. Problema

Verificado em execução na fase 0:

1. Reimportar o mesmo arquivo duplica todos os eventos (6 linhas viraram 12).
2. Erros de gravação viram só um contador (`Persistence errors: N`), sem linha nem motivo.
3. Linhas em branco viram eventos `event received without message`.
4. O categorizador testa `auth`/`login` antes dos sinais de segurança: `unauthorized access attempt` e `login failed` viram `audit`; o ramo `security` é inalcançável para esses casos.
5. `tracelog seed` é um stub documentado como se populasse dados.
6. Não há dados de exemplo nem demonstração reproduzível.

## 2. Objetivos

- Reimportar um arquivo não cria duplicatas e o resumo diz quantas linhas já existiam.
- Erros de gravação mostram arquivo, linha e motivo.
- Dataset sintético pequeno, versionado e reproduzível, mais um gerador para volumes maiores.
- Uma demonstração de ponta a ponta com um comando.

## 3. Não objetivos

- Lote/transação no import: continua um `INSERT` por linha (ver medição no `PROGRESS.md`).
- Detectar duplicatas entre arquivos de nomes diferentes.
- Novos formatos de linha.

## 4. Comportamento e contrato

### 4.1 `external_id` derivado

Cada linha importada recebe:

```text
external_id = "file:" + hex(sha256(basename(arquivo) + "\x00" + número_da_linha + "\x00" + linha_bruta))[:32]
```

- Usa o **nome base** do arquivo para que o mesmo arquivo importado de caminhos diferentes (host, container) gere os mesmos IDs.
- Com a deduplicação por (`application_id`, `external_id`) da spec 0002, reimportar não grava nada novo.
- Limite aceito: dois arquivos com o mesmo nome base e a mesma linha, no mesmo número, são tratados como o mesmo evento. Um arquivo que cresceu (log com linhas novas no fim) importa só as novas; um arquivo reescrito do início, com conteúdo diferente nas mesmas posições, importa as linhas diferentes.

### 4.2 Linhas em branco

Linhas vazias ou só com espaços não viram evento e são contadas em `Blank lines skipped`.

### 4.3 Resumo

```text
Log import summary
Files processed: N
Lines read: N
Persisted: N
Duplicates (already imported): N
Blank lines skipped: N
Normalized: N
Partially interpreted: N
Uncategorizable: N
Persistence errors: N        (só se N > 0)
  <arquivo>:<linha>: <motivo> (até 10 exemplos)
```

`Persisted` conta só linhas novas; `Normalized`, `Partially interpreted` e `Uncategorizable` se referem a elas. Erros acima de 10 aparecem só no total. O motivo é a mensagem do erro: a CLI é usada pelo operador local, que precisa dela para corrigir o problema (diferente do HTTP, spec 0002).

### 4.4 Categorizador

Ordem nova (a primeira que casar vence), com **palavras inteiras**, como a severidade:

1. `security`: `unauthorized`, `forbidden`, `denied`, `intrusion`, `security`, `credential(s)`, `token(s)`, ou uma palavra de autenticação (`login`, `logon`, `signin`, `auth`, `authentication`) junto com uma de falha (`failed`, `failure`, `invalid`, `rejected`, `locked`).
2. `audit`: `login`, `logout`, `auth`, `authentication`, `permission`, e as expressões `access granted`, `user action`, `created by`, `updated by`.
3. `system`: `database`, `disk`, `memory`, `cpu`, `network`, `timeout(s)`, `connection(s)`, `postgres`, `redis`, `kafka`.
4. Demais regras inalteradas.

### 4.4.1 Parser e status (decididos ao gerar o dataset)

- Linha que começa com `[` só vai para o parser JSON se for JSON válido. Assim `[06/Mar/2026:02:26:01 +0000] WARN ...` chega ao parser `timestamp nível mensagem` (que guarda o nível e registra o timestamp ilegível em `parse_error`), em vez de virar texto puro.
- O categorizador não promove mais `partial` para `normalized`. `normalized` significa que timestamp, mensagem e categoria foram reconhecidos; uma linha sem timestamp utilizável continua `partial`, mesmo com categoria.

### 4.5 `seed`

O comando `tracelog seed` e sua documentação são removidos. Dados de demonstração vêm de `examples/logs/` via `import logs` (ou `make demo`).

### 4.6 Dataset e gerador

- `tools/loggen`: gerador em Go, semente fixa (padrão 42), datas fixas (padrão 2026-03-01 a 2026-03-14), aplicação fictícia `checkout-api`, IPs só das faixas de documentação (RFC 5737), usuários `user-NNNN`. Nenhum dado real.
- `examples/logs/`: saída do gerador com os padrões, versionada. Um teste garante que os arquivos versionados são exatamente a saída do gerador.
- Três formatos, um por arquivo: JSON por linha, `timestamp nível mensagem` e texto livre; inclui um pico de erros num dia, falhas de login, linhas quebradas, sem timestamp e em branco.
- Volumes maiores: `go run ./tools/loggen -scale N -out <dir>`; não são versionados.

### 4.7 Demonstração

`scripts/demo.sh` (e `make demo`): sobe o compose, importa `examples/logs/` com a CLI dentro do container, envia eventos por `curl`, roda `events list` e `metrics errors`. Só exige Docker e `curl`.

## 5. Schema

Nenhuma mudança.

## 6. Critérios de aceite

1. Importar `examples/logs/` duas vezes: a segunda importação tem `Persisted: 0` e `Duplicates` igual às linhas não vazias.
2. Um erro de gravação mostra `arquivo:linha: motivo`; com mais de 10 erros, só 10 exemplos.
3. Linhas em branco não são gravadas e aparecem em `Blank lines skipped`.
4. `unauthorized access attempt` e `login failed for user-0001` → `security`; `user login succeeded` → `audit`.
5. `go run ./tools/loggen -out <tmp>` gera arquivos idênticos aos de `examples/logs/`.
6. `tracelog seed` não existe mais.
7. `make demo` termina com sucesso a partir de um `.env` copiado do exemplo.

## 7. Questões em aberto

Nenhuma.
