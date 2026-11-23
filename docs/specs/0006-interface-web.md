# Spec 0006: Interface web (dashboard)

- **Status:** Aprovada (todas as recomendações da seção 8 confirmadas pelo autor em 2026-11-23)
- **Autor:** Vitor Fábio
- **Data:** 2026-11-23
- **Escopo:** diretório `web/` (React + TypeScript), como o binário Go serve a UI, job de CI do front, cabeçalhos de segurança da UI
- **Relacionadas:** [0005](./0005-api-de-leitura.md) (API consumida; esta spec não muda o contrato dela), [0003](./0003-container-e-migrations.md) (imagem e compose)

## 1. Problema

A API de leitura (spec 0005) existe, mas investigar eventos ainda exige `curl` ou CLI. O objetivo do ciclo é um dashboard que mostre, a partir dos dados de exemplo, o que falhou, quando e com que detalhe, sem enfraquecer o que o projeto já garante: uma origem só, nenhum dado não confiável interpretado como HTML, nada exposto além de `127.0.0.1` por padrão.

Fatos medidos que restringem as decisões (Fase 3 da Trilha 2, `PROGRESS.md`):

1. **O Go percorre `node_modules`.** `go vet ./...` compila qualquer `.go` dentro de `web/node_modules`. Isso não é teórico: o pacote `flatted` (dependência do ESLint via `flat-cache`) publica `golang/pkg/flatted/flatted.go`. Um `go.mod` mínimo em `web/` torna o diretório um módulo à parte e o tira de `./...` (testado: `go vet ./...` volta a passar).
2. **O `jsdom` mais recente (31.0.1) exige Node `^22.22.2 || ^24.15.0 || >=26`.** A máquina do autor tem Node 24.13.0. O `happy-dom` 21 exige `>=20`.
3. **`GET /events` responde `405`** e precisa continuar assim (spec 0005). A UI não pode ter uma rota de cliente `/events` na raiz do mesmo binário.
4. O dataset de exemplo tem **uma aplicação e um ambiente** e o período útil é **2026-03-01 a 2026-03-14** (`data-range` já sugere esse período).

## 2. Objetivos

- Três telas: **Visão geral**, **Eventos** e **Detalhe do evento**, funcionando contra os dados de exemplo.
- Estado das telas na URL: links compartilháveis, voltar e avançar do navegador funcionando.
- Texto de log sempre renderizado como texto, com teste automatizado de um evento com `<script>`.
- Estados úteis de carregando, vazio e erro (API fora, `400` com o campo, `404`).
- Acessibilidade e responsividade básicas.
- Lint, typecheck, testes e build do front no CI, sem alterar os jobs Go existentes.
- `docker compose up` entrega API e dashboard numa origem só.

## 3. Não objetivos

Autenticação, tempo real (WebSocket/polling automático), alertas, busca textual, edição de dados, i18n, tema escuro, gráficos além do de barras empilhadas, filtro por `trace_id`/`correlation_id` (a API não tem), testes E2E, SSR, PWA/offline, mudanças na API de leitura (se uma tela exigir, a spec 0005 é atualizada antes).

## 4. Comportamento

### 4.1 Rotas da UI

A UI fica sob **`/ui/`** no mesmo binário (decisão (b)):

| Rota | Tela | Estado na query string |
|------|------|------------------------|
| `/ui/` | Visão geral | `from`, `to`, `application_id`, `environment_id` |
| `/ui/events` | Eventos | `severity`, `application_id`, `environment_id`, `from`, `to`, `cursor` |
| `/ui/events/:id` | Detalhe | — |
| `/` | — | `302` para `/ui/` |

- Nomes e formatos dos parâmetros são os mesmos da API: a URL da UI se traduz diretamente na chamada (`/ui/events?severity=error` → `GET /api/v1/events?severity=error&limit=50`).
- Parâmetro ausente não é escrito na URL. Na Visão geral, sem `from`/`to`, a tela usa `default_from`/`default_to` de `data-range` e mostra o período usado; a URL só muda quando o usuário escolhe um período.
- Valor inválido digitado na URL chega à API, que responde `400` com `field`: a tela mostra a mensagem junto do filtro e um link "limpar filtros". A UI não duplica as regras de validação da API.

### 4.2 Visão geral

- Filtros: período (`from`, `to`, dois `<input type="date">`), aplicação e ambiente (`<select>` com "Todas"/"Todos", nomes de `/api/v1/applications` e `/environments`).
- Gráfico de **barras empilhadas por dia UTC**: `errors` e `fatal`, de `/api/v1/metrics/error-frequency`. Cada barra é um link para `/ui/events?from=<dia>&to=<dia>` (com os mesmos filtros de aplicação/ambiente). O gráfico tem rótulo acessível e uma tabela equivalente ("ver como tabela").
- Contadores por severidade (`debug`, `info`, `warn`, `error`, `fatal` e total) de `/api/v1/metrics/severity-counts` no mesmo período e filtros, cada um com link para a lista filtrada por aquela severidade.
- Período acima de 366 dias: a API responde `400`; a tela mostra a mensagem.

### 4.3 Eventos

- Filtros: severidade, aplicação, ambiente, `from`, `to`. Aplicar filtros remove o `cursor` da URL.
- Tabela: `occurred_at` (UTC), severidade, categoria, mensagem (cortada visualmente com reticências; texto completo no `title` e no detalhe), aplicação (nome), canal. Cada linha leva ao detalhe por um link na mensagem.
- Página fixa de 50 (o padrão da API); sem seletor de tamanho no MVP.
- **Paginação:** "Próxima página" faz *push* de `cursor=<next_cursor>` na URL; "Primeira página" remove o `cursor`. A página anterior é o **voltar do navegador** (cada página é uma entrada de histórico). Sem `next_cursor`, "Próxima" fica desabilitada.
- Estado vazio: "Nenhum evento para estes filtros", com link para limpar.

### 4.4 Detalhe do evento

- Todos os campos do `GET /api/v1/events/{id}`, com nomes de aplicação, ambiente e fonte resolvidos pelas rotas de referência (UUID visível ao lado).
- `occurred_at` e `received_at` lado a lado (são horários diferentes de propósito, guia §7.1).
- Avisos de `payload._ingestion` em destaque: `normalization_warnings` como lista, `decode_error`, `parse_error` e um aviso explícito quando houver `raw_request_body` ("o corpo recebido não era JSON válido").
- `payload` como JSON formatado (`JSON.stringify(payload, null, 2)`) dentro de `<pre>`; `raw_line` e `raw_request_body` em `<pre>` separados.
- `trace_id` e `correlation_id` exibidos e copiáveis; sem link, porque a API não filtra por eles.
- `404`: "Evento não encontrado", com link para a lista.

### 4.5 Horários e textos

- Todos os horários exibidos em **UTC**, com o sufixo "UTC", porque os dias do gráfico e os filtros `YYYY-MM-DD` são UTC (decisão (h)).
- Textos da interface em **inglês** (decisão (i)).

### 4.6 Erros e estados

| Situação | Comportamento |
|----------|---------------|
| Carregando | Esqueleto/indicador com `aria-busy`, sem deslocar o layout. |
| Rede falhou ou resposta não é JSON (ex.: proxy sem API) | "Não foi possível falar com a API" + botão "tentar de novo". |
| `400` | Mensagem da API (`error`) junto do filtro `field`, e "limpar filtros". |
| `404` | Mensagem específica da tela. |
| `500` | "Erro interno do servidor" + "tentar de novo"; o corpo da API já é genérico. |
| Vazio | Texto específico por tela. |

O cliente HTTP é um módulo só (`web/src/api/`), com tipos TypeScript escritos à mão a partir da spec 0005 e um tipo de erro que carrega `status`, `code` e `field`.

### 4.7 Segurança

- **Nunca** `dangerouslySetInnerHTML`, `innerHTML`, `eval` ou `new Function`. Garantido por regra do ESLint (`no-restricted-syntax` e `no-restricted-properties`, regras nativas, sem plugin extra).
- Nenhum valor vindo de evento vira `href`, `src` ou atributo de estilo: só texto. Links da UI são montados com IDs validados pela própria API e com `encodeURIComponent`.
- Quando servida pelo Go, as respostas de `/ui/` levam:
  - `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'`
  - `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`.
- Sem CORS (a spec 0005 não habilita): em dev o proxy do Vite, em produção a mesma origem.

### 4.8 Acessibilidade e responsividade

- Todo controle com `<label>` associado; foco visível; tudo operável por teclado; `lang="en"`; títulos de página por rota.
- Tabela semântica (`<table>`, `<th scope>`); severidade nunca só por cor (texto sempre presente).
- Contraste AA (≥ 4,5:1 para texto) nas cores escolhidas, conferido por cálculo na fase 5.
- Telas utilizáveis a partir de 360 px de largura: filtros empilham, a tabela rola horizontalmente.

## 5. Contrato

### 5.1 Servir a UI pelo binário (decisão (a))

| Rota | Resposta |
|------|----------|
| `GET /` | `302` para `/ui/` |
| `GET /ui/assets/*` | Arquivo do build. Nomes com hash: `Cache-Control: public, max-age=31536000, immutable`. Inexistente → `404` (nunca o `index.html`, para não servir HTML como JS). |
| `GET /ui/` e qualquer outro `GET /ui/*` | `index.html` da SPA, `Cache-Control: no-cache`, cabeçalhos da seção 4.7. |
| Outros métodos em `/ui/*` | `405` com `Allow: GET, HEAD` |
| Binário compilado sem o build do front | `/ui/` responde uma página estática curta: "UI not built: run `make web-build`" (placeholder versionado). |

- O build do Vite (`base: '/ui/'`) escreve num diretório **do módulo Go raiz** (proposto: `internal/adapters/http/webui/dist/`, ignorado pelo Git), embutido com `go:embed` junto de um placeholder versionado. `go build` funciona sem Node.
- `web/go.mod` (arquivo mínimo, comentado) mantém `web/node_modules` fora do `go ./...` (fato 1 da seção 1).
- Rotas existentes (`/health`, `/ready`, `/events`, `/api/`) não mudam.

### 5.2 Estrutura de `web/`

```text
web/
├── go.mod                 # só para o Go ignorar web/ (ver 5.1)
├── package.json, package-lock.json, .nvmrc
├── index.html, vite.config.ts, tsconfig.json, eslint.config.js
└── src/
    ├── api/               # cliente HTTP único + tipos da spec 0005
    ├── routes/            # Overview, Events, EventDetail
    ├── components/        # gráfico SVG, tabela, filtros, estados
    └── lib/               # filtros <-> query string, datas UTC
```

### 5.3 Comandos

Novos alvos no `Makefile`, sem alterar os existentes: `web-install` (`npm ci`), `web-lint`, `web-typecheck`, `web-test`, `web-build`, `web-dev` (Vite com proxy de `/api` e `/ready` para `http://127.0.0.1:${HTTP_PORT}`).

### 5.4 Dependências propostas

Versões atuais no registro npm em 2026-11-23. Tamanho "publicado" é o `unpackedSize` do pacote (o que vai para `node_modules`), **não** o custo no bundle; o bundle será medido no build (gzip) na fase 4 e a cada tela na fase 5.

| Pacote | Versão | Uso | Publicado | Por quê / alternativa |
|--------|--------|-----|-----------|------------------------|
| `react`, `react-dom` | 19.3.1 | runtime | 174 KB, 7,7 MB | Stack pedida. |
| `react-router` | 8.5.2 | runtime | 2,9 MB | Rotas, `useSearchParams`, histórico. Alternativa: roteador próprio sobre a History API (~60 linhas, zero dependência), viável com 3 rotas; recomendo a biblioteca pela navegação e testes prontos, revisável se o bundle medido pesar. |
| `@tanstack/react-query` | 5.108.4 | runtime | 1,7 MB | Cache por chave (filtros), estados de carregando/erro, *retry*, sem `useEffect` manual em cada tela. Alternativa: hook próprio de `fetch`, que reimplementaria cache e cancelamento. |
| `tailwindcss`, `@tailwindcss/vite` | 4.3.5 | build | 769 KB | CSS gerado só com as classes usadas, sem CSS-in-JS (compatível com a CSP sem `unsafe-inline`). |
| `vite`, `@vitejs/plugin-react` | 8.3.8, 6.3.0 | build | 2,3 MB | Dev server com proxy e build. |
| `typescript` | 7.0.2 | build | 2,4 MB | `strict: true`. |
| `vitest` | 5.1.1 | teste | 2,8 MB | Integra com a config do Vite. |
| `@testing-library/react`, `@testing-library/user-event` | 16.3.3, 14.6.13 | teste | 332 KB, 430 KB | Testes pelo que o usuário vê; teclado. Sem `@testing-library/jest-dom` (os *matchers* extras não são necessários). |
| `happy-dom` | 21.0.4 | teste | 8,8 MB | DOM dos testes; ver decisão (e). |
| `eslint`, `typescript-eslint`, `eslint-plugin-react-hooks` | 10.15.0, 8.74.0, 7.1.1 | lint | 2,8 MB, 42 KB, 3,9 MB | Lint com tipos e regras de hooks. Sem Prettier (decisão (g)). |
| `@types/react`, `@types/react-dom` | 19.3.0, 19.3.0 | build | 399 KB, 32 KB | Tipos. |

**Não entra:** Recharts (decisão (c)), biblioteca de componentes, de datas (as datas são `YYYY-MM-DD` UTC, tratáveis com `Date.UTC`), de ícones, axios (o `fetch` basta).

### 5.5 CI

Job novo `web` em `.github/workflows/ci.yml`, sem alterar os jobs existentes: `actions/checkout` e `actions/setup-node` fixadas por SHA, `node-version-file: web/.nvmrc`, cache do npm por `web/package-lock.json`, `permissions: contents: read`; passos `npm ci`, lint, typecheck, test, build. Quando a UI for embutida (fase 6), o job `docker build` existente passa a construir também o front pelo `Dockerfile`; essa mudança no job entra na fase 6, com a medição de tamanho.

## 6. Mudanças de schema

Nenhuma.

## 7. Critérios de aceite

1. As três telas funcionam no navegador contra a API real com `examples/logs` importado; a Visão geral abre em 2026-03-01 a 2026-03-14 sem o usuário escolher período, e mostra o pico de 2026-03-10 (25 error + 1 fatal).
2. Filtros e cursor vivem na URL: recarregar a página mantém o estado; voltar do navegador retorna à página anterior da lista; colar a URL em outra aba reproduz a tela.
3. Teste automatizado: um evento cuja mensagem e `payload` contêm `<script>alert("xss")</script>` e `<img src=x onerror=alert(1)>` (fixture `event_detail.json` da spec 0005) aparece como texto e não cria elemento `script` nem `img` no DOM.
4. Testes automatizados para: cliente HTTP (sucesso, `400` com `field`, `404`, `500`, rede fora, resposta não JSON), conversão filtros ↔ query string (ida e volta, ausentes não escritos), estados carregando/vazio/erro de cada tela. Os mocks usam as fixtures de `internal/adapters/http/testdata/api/`.
5. O ESLint falha num arquivo de teste com `dangerouslySetInnerHTML` (regra provada por teste ou execução registrada).
6. `go vet ./...` e `go test ./...` passam com `web/node_modules` instalado.
7. `go build` funciona sem o build do front; `/ui/` mostra o placeholder.
8. Com a UI embutida: `/ui/*` devolve os cabeçalhos da seção 4.7; o console do navegador não mostra violações de CSP nas três telas.
9. Navegação completa por teclado nas três telas; todos os controles com rótulo (verificado pela árvore de acessibilidade); uso em 360 px de largura sem rolagem horizontal da página (só da tabela).
10. Job `web` verde no CI; `make lint`, `make test` e `make build` continuam como antes.
11. Tamanho do bundle (gzip) e da imagem Docker medidos e registrados com o método, antes e depois do embed.

## 8. Decisões para o autor

Recomendação primeiro em cada item.

**(a) Como servir.** **Embutido no binário** (`go:embed`), uma origem só.
- Por quê: sem CORS e sem segundo serviço; um binário entrega tudo, como o roadmap prevê ("UI web embutida"); a CSP `'self'` fica simples.
- Custos:
  - estágio Node no `Dockerfile`. É só de build, então a imagem final cresce apenas o tamanho do `dist` embutido no binário (estimativa a confirmar: centenas de KB sobre os 31,5 MB atuais; medido na fase 6);
  - um placeholder versionado para o `go build` não depender do Node;
  - o job `docker build` passa a construir o front.
- Descartado: serviço separado com nginx no compose. Ele separa os deploys, mas cria uma segunda origem: exigiria CORS na API (que a spec 0005 decidiu não ter) ou o nginx fazendo proxy de `/api`. Também acrescenta mais uma imagem (tamanho não medido) e não serve quem roda o binário sem compose.

**(b) Caminho da UI.** **`/ui/`**, com `/` redirecionando.
- Por quê: separa estáticos, API (`/api/`) e ingestão (`/events`) por prefixo, resolve o conflito com `GET /events` (fato 3 da seção 1), e o fallback da SPA fica restrito a `/ui/*`.
- Descartado: UI na raiz com rotas de cliente renomeadas (ex.: `/explore`). Daria URLs mais curtas, mas o fallback na raiz teria de excluir uma lista de caminhos reservados que cresce a cada rota nova.

**(c) Gráfico.** **SVG próprio**: um componente de barras empilhadas com eixo de dias, eixo de contagem, legenda e tabela acessível, estimado em 100 a 150 linhas.
- Por quê: um único gráfico simples. Recharts 3 traz 11 dependências (entre elas `@reduxjs/toolkit`, `react-redux`, `immer`, `victory-vendor`/d3) e 8,5 MB publicados, para desenhar duas séries.
- Custo: o código e a acessibilidade ficam conosco, testados.
- Descartado: Recharts. É conhecido e tem tooltips e animação prontos. Revisável se o dashboard ganhar mais gráficos (item 6 do roadmap).
- O bundle será medido com o SVG; não vou medir o Recharts instalando-o só para comparar, a menos que você peça.

**(d) Roteamento e dados.** **React Router + TanStack Query**, como na stack proposta.
- Justificativa e alternativas na tabela 5.4. Se o bundle medido na fase 4 mostrar peso desproporcional, a alternativa (roteador próprio) volta para decisão.

**(e) DOM dos testes.** **happy-dom**, ou atualizar o Node local para 24.15+ e usar jsdom.
- Por quê: o jsdom atual não roda no Node 24.13.0 da sua máquina (fato 2 da seção 1).
- Custo do happy-dom: implementa menos da especificação do DOM que o jsdom, o que é suficiente para Testing Library em componentes, sem layout.
- Alternativas:
  - Node 24.15+ local e no CI, com jsdom, que é a referência mais fiel;
  - jsdom 29, a última versão compatível com o seu Node, mas desatualizada.

**(f) Versão do Node.** **`web/.nvmrc` com `24.13.0` exato** (a sua versão, LTS da linha 24) e `engines` no `package.json`, mais a imagem `node:24.13.0-alpine` no `Dockerfile` (tag a conferir na fase 6).
- Por quê: mesma disciplina do Go (`1.26.11` exato), com CI, Docker e máquina local iguais.
- Custo: atualizar a versão à mão.
- Descartado: `24` sem patch (CI e local divergem sem aviso).

**(g) Lint e formatação.** **ESLint (typescript-eslint + react-hooks), sem Prettier**; `tsc --noEmit` como typecheck.
- Por quê: o ESLint pega bugs e aplica a regra de segurança da seção 4.7. O Prettier seria a terceira ferramenta só para estilo, e o Go já tem `gofmt`.
- Descartado: Prettier ou Biome. São úteis em equipe; aceito se você preferir.

**(h) Fuso exibido.** **UTC sempre**.
- Por quê: os dias do gráfico, os filtros `YYYY-MM-DD` e a API são UTC; misturar fuso local faria um evento de 22:00 em São Paulo aparecer no dia seguinte do gráfico e no dia anterior da tabela.
- Descartado: horário local com o UTC no `title`. É mais familiar, mas inconsistente com os filtros.

**(i) Idioma da interface.** **Inglês**.
- Por quê: as mensagens da CLI e da API e o README estão em inglês, e a UI repete mensagens da API (`error`). A documentação continua em português.
- Descartado: português (o idioma do guia), que misturaria idiomas na mesma tela, porque as mensagens de erro da API viriam em inglês.

**(j) Estado na URL e paginação.** Filtros e `cursor` na query string; "anterior" pelo histórico do navegador; "Primeira página" explícita.
- Por quê: links compartilháveis e o voltar funcionando, sem guardar uma pilha de cursores.
- Custo: quem abre um link com `cursor` não tem "anterior" dentro da UI, só "Primeira página".
- Descartado: pilha de cursores em memória. Tem "anterior" de verdade, mas se perde no reload e diverge do histórico.

**(k) Fixtures compartilhadas.** Sim, como aprovado na spec 0005 §5.4: os testes do front importam `internal/adapters/http/testdata/api/*.json` por um alias do Vite (`@fixtures`).
- Custo: o front lê um diretório fora de `web/`, então o `.dockerignore` e o contexto do build precisam incluir esse caminho só no estágio de teste. O build de produção não usa as fixtures.

## 9. Questões em aberto

Nenhuma bloqueia a implementação; as três abaixo são decididas nas fases indicadas.

1. **Dataset com uma aplicação só** (spec 0005, questão 2): os seletores de aplicação e ambiente terão uma opção cada com os dados de exemplo. Recomendo decidir antes da fase 6 (demo); não bloqueia esta spec.
2. **`Dockerfile` e contexto de build com `web/`**: o `.dockerignore` atual precisa excluir `web/node_modules` e `web/dist`. É detalhe da fase 6, registrado aqui para não ser esquecido.
3. **GIF e prints (fase 6):** não há Playwright instalado. Instalar exige um navegador (download grande). Alternativa: capturas pelo navegador embutido do app ou um roteiro do que capturar. Decidir na fase 6.
