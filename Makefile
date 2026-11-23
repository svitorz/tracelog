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
