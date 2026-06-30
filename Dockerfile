# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.1

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kotoba-ssh ./cmd/kotoba-ssh
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kotoba-web ./cmd/kotoba-web

FROM alpine:3.22
WORKDIR /app

ENV PORT=8080 \
    KOTOBA_HTTP_HOST=0.0.0.0 \
    KOTOBA_STATE_DIR=/data

RUN apk add --no-cache ca-certificates && mkdir -p /data

RUN <<'EOF'
cat > /usr/local/bin/kotoba-railway-start <<'SCRIPT'
#!/bin/sh
set -eu

export KOTOBA_HTTP_HOST="${KOTOBA_HTTP_HOST:-0.0.0.0}"
export KOTOBA_HTTP_PORT="${KOTOBA_HTTP_PORT:-${PORT:-8080}}"
export KOTOBA_STATE_DIR="${KOTOBA_STATE_DIR:-/data}"

exec /app/kotoba-web
SCRIPT
chmod +x /usr/local/bin/kotoba-railway-start
EOF

COPY --from=build /out/kotoba-ssh /app/kotoba-ssh
COPY --from=build /out/kotoba-web /app/kotoba-web
COPY content /app/content

EXPOSE 8080/tcp

CMD ["/usr/local/bin/kotoba-railway-start"]
