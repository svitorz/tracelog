// Este arquivo não tem código Go: ele só faz de web/ um módulo à parte, para que
// `go vet ./...` e `go test ./...` na raiz não entrem em web/node_modules (alguns
// pacotes npm publicam arquivos .go; spec 0006, seção 5.1).
module svitorz/tracelog/web

go 1.26.11
