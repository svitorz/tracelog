# syntax=docker/dockerfile:1

# Build: Go igual ao go.mod; GOTOOLCHAIN=local impede download de outro toolchain.
FROM golang:1.26.11-alpine3.24 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/tracelog .

# Runtime: alpine pelo wget do healthcheck (spec 0003); binário estático, usuário sem root.
FROM alpine:3.24
RUN addgroup -S -g 10001 tracelog && adduser -S -D -H -u 10001 -G tracelog tracelog
COPY --from=build /out/tracelog /usr/local/bin/tracelog

# Sem .env na imagem: a configuração vem só de variáveis de ambiente.
WORKDIR /app
USER 10001:10001
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${HTTP_PORT:-8080}/ready" || exit 1

ENTRYPOINT ["tracelog"]
CMD ["serve"]
