# Railway SSH Deployment

This page prepares Kotoba Line for issue #10. It does not create Railway
resources; use it when the Go SSH app exists and the project is ready for a
hosted smoke test.

## Runtime Contract

- Binary: `/app/kotoba-ssh`.
- Container start wrapper: `/usr/local/bin/kotoba-railway-start`.
- Effective bind address: `0.0.0.0:${PORT:-2222}`.
- Persistent state directory: `/data`.
- Railway Volume mount path: `/data`.
- Replica count: `1`.
- SSH behavior: the service must start the Kotoba Line TUI directly. Do not
  expose shell, exec, SFTP, or subsystem handlers to SSH clients.
- Password: set `KOTOBA_SSH_PASSWORD` explicitly for Railway. The development
  default is rejected when the app binds to a non-local host.

The Dockerfile sets `PORT=2222`, `KOTOBA_SSH_HOST=0.0.0.0`,
`KOTOBA_SSH_HOST_KEY_PATH=/data/ssh_host_ed25519`, and `KOTOBA_STATE_DIR=/data`
as image defaults. Its start wrapper maps `KOTOBA_SSH_PORT` from
`${PORT:-2222}` when `KOTOBA_SSH_PORT` is not set. Railway service variables can
override these values. The application should treat `/data` as the only
persisted state path on Railway.

## Repository Configuration

- `Dockerfile` builds `./cmd/kotoba-ssh` into `/app/kotoba-ssh`.
- Dockerfile cache mounts must include explicit `id=` values for Railway's
  builder.
- Do not add a Docker `VOLUME` instruction for `/data`; Railway rejects Docker
  volumes and expects the service Volume resource plus `requiredMountPath`.
- `railway.toml` selects Railway's Dockerfile builder, starts
  `/usr/local/bin/kotoba-railway-start`, requires a `/data` mount, and pins the
  service to one replica.
- `.dockerignore` excludes local state, local env files, git metadata, and build
  output from the Docker context.

Railway's TCP Proxy and Volume are service resources. They are not created by
this repo config.

## GitHub Deploy Flow

1. In Railway, create a project from the public GitHub repo
   `superposition/kotoba-line`.
2. Use the repo root as the service root so Railway reads `railway.toml`.
3. Keep GitHub deploys enabled for the branch Railway should follow.
4. Add a Railway Volume attached to the SSH service and mount it at `/data`.
5. Keep the service at a single replica. The prototype event log is local-file
   state, and Railway volumes do not support multiple mounted replicas.
6. Confirm the service variables:

   ```sh
   PORT=2222
   KOTOBA_SSH_HOST=0.0.0.0
   KOTOBA_SSH_PASSWORD=<private-password>
   KOTOBA_SSH_HOST_KEY_PATH=/data/ssh_host_ed25519
   KOTOBA_STATE_DIR=/data
   ```

7. Enable TCP Proxy for the service. Enter internal port `2222`, unless the
   service uses a different `PORT`.
8. Connect with the Railway-generated proxy host and port:

   ```sh
   ssh player@<proxy-domain> -p <proxy-port>
   ```

## Lightweight Validation

Run these before creating Railway resources:

```sh
python3 - <<'PY'
import pathlib
import tomllib

config = tomllib.loads(pathlib.Path("railway.toml").read_text())
assert config["build"]["builder"] == "DOCKERFILE"
assert config["deploy"]["startCommand"] == "/usr/local/bin/kotoba-railway-start"
assert config["deploy"]["numReplicas"] == 1
assert config["deploy"]["requiredMountPath"] == "/data"
print("railway.toml ok")
PY
```

```sh
docker build --check .
```

When the Go app exists, also run:

```sh
docker build -t kotoba-line:ssh .
docker run --rm -p 2222:2222 \
  -e KOTOBA_SSH_PASSWORD=local-test-password \
  -v kotoba-line-data:/data \
  kotoba-line:ssh
```

In another terminal, verify the listener and SSH behavior:

```sh
lsof -nP -iTCP:2222 -sTCP:LISTEN
ssh player@127.0.0.1 -p 2222
ssh player@127.0.0.1 -p 2222 'echo shell access should fail'
```

The listener must be reachable through Docker's published port, the interactive
SSH command should open the TUI, and the exec-style command must not provide a
shell.

After Railway resources are created, issue #10 closeout evidence should include:

```sh
railway status
railway logs
ssh player@<proxy-domain> -p <proxy-port>
```

Record the exact proxy host, proxy port, deploy URL, and persistence check.

## References

- Railway Config as Code:
  <https://docs.railway.com/config-as-code/reference>
- Railway TCP Proxy:
  <https://docs.railway.com/networking/tcp-proxy>
- Railway Volumes:
  <https://docs.railway.com/volumes/reference>

## Attempt Logs

- [Wave 002 Worker C](./railway-wave-002-worker-c.md)
- [Wave 003 Worker D](./railway-wave-003-worker-d.md)
- [Wave 004 Worker E](./railway-wave-004-worker-e.md)
