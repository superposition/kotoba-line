# Railway Web Deployment

This page prepares Kotoba Line for issue #10. It does not create Railway
resources; use it when the Go browser app exists and the project is ready for a
hosted smoke test.

## Runtime Contract

- Binary: `/app/kotoba-web`.
- Container start wrapper: `/usr/local/bin/kotoba-railway-start`.
- Effective bind address: `0.0.0.0:${PORT:-8080}`.
- Persistent state directory: `/data`.
- Railway Volume mount path: `/data`.
- Hosted state database: set `KOTOBA_DATABASE_URL` to the Railway Postgres
  `DATABASE_URL` reference for production user/progress persistence. SQLite is
  the local fallback.
- Replica count: `1`.
- Web behavior: the service must serve the Kotoba Line gameplay UI through the
  Railway public HTTPS domain.
- Password: set `KOTOBA_SSH_PASSWORD` explicitly for Railway. The development
  default is rejected when the app binds to a non-local host.
- Users: set `KOTOBA_SSH_USERS` to a comma-separated list when multiple
  usernames should share the same password while keeping per-user progress.
- User password overrides: set `KOTOBA_SSH_USER_PASSWORDS` to comma-separated
  `username=password` pairs when one configured user needs a different password.

The Dockerfile sets `PORT=8080`, `KOTOBA_HTTP_HOST=0.0.0.0`, and
`KOTOBA_STATE_DIR=/data` as image defaults. Its start wrapper maps
`KOTOBA_HTTP_PORT` from `${PORT:-8080}` when `KOTOBA_HTTP_PORT` is not set.
Railway service variables can override these values. The application should
treat Postgres as the production progress store and `/data` as fallback volume
state.

## Repository Configuration

- `Dockerfile` builds `./cmd/kotoba-web` into `/app/kotoba-web`.
- Dockerfile cache mounts must include explicit `id=` values for Railway's
  builder.
- Do not add a Docker `VOLUME` instruction for `/data`; Railway rejects Docker
  volumes and expects the service Volume resource plus `requiredMountPath`.
- `railway.toml` selects Railway's Dockerfile builder, starts
  `/usr/local/bin/kotoba-railway-start`, requires a `/data` mount, and pins the
  service to one replica.
- `.dockerignore` excludes local state, local env files, git metadata, and build
  output from the Docker context.

Railway's public domain, Postgres service, and Volume are service resources.
They are not created by this repo config.

## GitHub Deploy Flow

1. In Railway, create a project from the public GitHub repo
   `superposition/kotoba-line`.
2. Use the repo root as the service root so Railway reads `railway.toml`.
3. Keep GitHub deploys enabled for the branch Railway should follow.
4. Add a Railway Volume attached to the app service and mount it at `/data`.
5. Keep the service at a single replica unless all writable state is confirmed
   to use Postgres.
6. Confirm the service variables:

   ```sh
   PORT=8080
   KOTOBA_HTTP_HOST=0.0.0.0
   KOTOBA_SSH_USERS=logohere,thescoho,superposition
   KOTOBA_SSH_PASSWORD=<private-password>
   KOTOBA_SSH_USER_PASSWORDS=superposition=kotoba
   KOTOBA_DATABASE_URL=${{Postgres.DATABASE_URL}}
   KOTOBA_STATE_DIR=/data
   ```

7. Ensure a Railway public domain exists for the service.
8. Open the Railway public HTTPS URL and log in as one of the configured users.

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
docker build -t kotoba-line:web .
docker run --rm -p 8080:8080 \
  -e KOTOBA_SSH_USERS=logohere,thescoho,superposition \
  -e KOTOBA_SSH_PASSWORD=local-test-password \
  -e KOTOBA_SSH_USER_PASSWORDS=superposition=kotoba \
  -v kotoba-line-data:/data \
  kotoba-line:web
```

In another terminal, verify the listener and HTTP behavior:

```sh
lsof -nP -iTCP:8080 -sTCP:LISTEN
curl -i http://127.0.0.1:8080/healthz
curl -i -c /tmp/kotoba-cookies.txt -b /tmp/kotoba-cookies.txt \
  -X POST http://127.0.0.1:8080/login \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data 'username=logohere&password=local-test-password'
```

The listener must be reachable through Docker's published port, the health check
must return `ok`, and login must return `303 See Other` with a session cookie.

After Railway resources are created, issue #10 closeout evidence should include:

```sh
railway status
railway logs
curl -i https://<railway-public-domain>/healthz
```

Record the exact public URL, deployment ID, and persistence check.

## References

- Railway Config as Code:
  <https://docs.railway.com/config-as-code/reference>
- Railway Public Networking:
  <https://docs.railway.com/networking/public-networking>
- Railway Volumes:
  <https://docs.railway.com/volumes/reference>

## Attempt Logs

- [Wave 002 Worker C](./railway-wave-002-worker-c.md)
- [Wave 003 Worker D](./railway-wave-003-worker-d.md)
- [Wave 004 Worker E](./railway-wave-004-worker-e.md)
