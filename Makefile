# Atalhos para os comandos do projeto. O CI chama os comandos go diretamente.

.PHONY: test lint build up migrate demo

test: ## testes com race detector (integração só com TRACELOG_TEST_DATABASE_URL)
	go test -race -count=1 ./...

lint: ## gofmt (falha se houver arquivo fora do padrão) e go vet
	@unformatted="$$(gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "Arquivos fora do padrão gofmt:"; echo "$$unformatted"; exit 1; fi
	go vet ./...

build: ## binário em bin/tracelog
	go build -o bin/tracelog .

up: ## banco, migrations, setup e API via docker compose
	@test -f .env || { echo "Falta o .env: rode cp .env.example .env"; exit 1; }
	docker compose up -d --build

migrate: ## aplica as migrations no banco do .env (CLI no host)
	go run . migrate

demo: ## sobe tudo, importa examples/logs, envia eventos e consulta (scripts/demo.sh)
	./scripts/demo.sh

# Front-end (web/, spec 0006). Exige Node da versão em web/.nvmrc.
.PHONY: web-install web-lint web-typecheck web-test web-build web-dev

web-install: ## instala as dependências do front exatamente como no package-lock.json
	cd web && npm ci

web-lint: ## ESLint, inclusive a regra contra HTML cru
	cd web && npm run lint

web-typecheck: ## tsc --noEmit (strict)
	cd web && npm run typecheck

web-test: ## Vitest + Testing Library
	cd web && npm test

web-build: ## build de produção em internal/adapters/http/webui/dist
	cd web && npm run build

web-dev: ## Vite em 127.0.0.1:5173 com proxy para TRACELOG_API_URL (padrão http://127.0.0.1:8080)
	cd web && npm run dev
