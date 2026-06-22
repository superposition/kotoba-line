# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.1

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kotoba-ssh ./cmd/kotoba-ssh

FROM alpine:3.22
WORKDIR /app

ENV PORT=2222 \
    KOTOBA_SSH_HOST=0.0.0.0 \
    KOTOBA_SSH_HOST_KEY_PATH=/data/ssh_host_ed25519 \
    KOTOBA_STATE_DIR=/data

RUN apk add --no-cache ca-certificates && mkdir -p /data

RUN <<'EOF'
cat > /usr/local/bin/kotoba-railway-start <<'SCRIPT'
#!/bin/sh
set -eu

export KOTOBA_SSH_HOST="${KOTOBA_SSH_HOST:-0.0.0.0}"
export KOTOBA_SSH_PORT="${KOTOBA_SSH_PORT:-${PORT:-2222}}"
export KOTOBA_SSH_HOST_KEY_PATH="${KOTOBA_SSH_HOST_KEY_PATH:-/data/ssh_host_ed25519}"
export KOTOBA_STATE_DIR="${KOTOBA_STATE_DIR:-/data}"

exec /app/kotoba-ssh
SCRIPT
chmod +x /usr/local/bin/kotoba-railway-start
EOF

COPY --from=build /out/kotoba-ssh /app/kotoba-ssh
COPY content /app/content

VOLUME ["/data"]
EXPOSE 2222/tcp

CMD ["/usr/local/bin/kotoba-railway-start"]
