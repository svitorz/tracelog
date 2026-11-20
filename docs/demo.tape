# Roteiro do GIF da demonstração, para o VHS (https://github.com/charmbracelet/vhs).
#
# Pré-requisitos: vhs instalado, .env copiado do .env.example e o stack no ar:
#   make up            # ou: docker compose up -d --wait
#   vhs docs/demo.tape # gera docs/demo.gif
#
# O stack precisa estar no ar antes porque o build da imagem deixaria o GIF longo.
# Rode com o banco vazio para o import mostrar "Persisted: 429" e não duplicatas.

Output docs/demo.gif

Require docker
Require curl

Set Shell "bash"
Set FontSize 15
Set Width 1280
Set Height 760
Set Padding 20
Set TypingSpeed 35ms
Set Theme "Catppuccin Mocha"

# Atalho invisível: a CLI roda dentro do container, com os exemplos montados.
Hide
Type `alias tracelog='docker compose --progress quiet run --rm --no-deps -T -v "$PWD/examples/logs:/examples:ro" api'`
Enter
Type "clear"
Enter
Show

Type "docker compose ps -a --format 'table {{.Service}}\t{{.Status}}'"
Enter
Sleep 2s

Type "tracelog import logs --file /examples/checkout-api.log --file /examples/auth-service.log --file /examples/legacy-gateway.log"
Enter
Sleep 6s

Type `curl -s -X POST localhost:8080/events -H 'Content-Type: application/json' -d '{"external_id":"gif-1","message":"payment provider timeout","occurred_at":"2026-03-14T18:00:00Z"}'`
Enter
Sleep 2s

Type "tracelog metrics errors --from 2026-03-01 --to 2026-03-14"
Enter
Sleep 6s
