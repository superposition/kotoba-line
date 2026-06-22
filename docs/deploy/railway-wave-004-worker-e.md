# Railway Wave 004 Worker E Attempt

Date: 2026-06-22

## Scope

Issue #10 hosted SSH deployment from current `main`, after Wave 004 gameplay
slices merged.

No secret values were printed or documented. Variable checks were limited to key
names.

## Current Main

- Repo: `https://github.com/superposition/kotoba-line`
- Branch: `main`
- Commit before Dockerfile fix: `2e121b7`
- Commit subject: `Merge pull request #20 from superposition/codex/wave-004-skill-gates`

## Railway State Rechecked

- CLI: `railway 4.59.0`
- Logged-in account: `Eric Manganaro (ericmanganaro@gmail.com)`
- Project: `kotoba-line`
  - ID: `b3d1fa80-9e54-4ec2-a88d-420bec09b3b7`
- Environment: `production`
  - ID: `88b447fa-9608-4c30-8f27-137819fa4311`
- Service: `kotoba-line-ssh`
  - ID: `1e700d31-3245-49d5-9087-f7d279198f52`
  - CLI-visible linked state: `isLinked: false`
  - CLI-visible source: `null`
  - Domains: none
  - Replicas configured: `1`
  - Replicas running: `0`
- Volume: `kotoba-line-ssh-volume`
  - ID: `025234ad-b46b-4461-b7f4-321970a1d7d5`
  - Mount path: `/data`
  - State: `READY`

Variable keys present:

- `KOTOBA_SSH_HOST`
- `KOTOBA_SSH_HOST_KEY_PATH`
- `KOTOBA_SSH_PASSWORD`
- `KOTOBA_STATE_DIR`
- `PORT`
- Railway-provided project/service/volume variables

## Source and TCP Proxy Attempt

`railway add --repo superposition/kotoba-line --service kotoba-line-ssh --json`
still returned:

```text
Unauthorized. Please run `railway login` again.
```

Railway Agent was then asked to connect `superposition/kotoba-line` branch
`main` and create a TCP Proxy for internal port `2222`. It reported success,
but normal Railway CLI status still showed:

- `source.repo: null`
- `isLinked: false`
- no service domains
- no custom domains

The next deployment behavior did change: Railway began reporting GitHub App
access as the source-fetch blocker.

## Dockerfile Build Fix

A fresh `railway up` against commit `2e121b7` created deployment
`b45cf412-5316-4d68-9641-737c94900e0f` and reached Dockerfile validation.

Build errors:

- `RUN --mount=type=cache,target=/go/pkg/mod` was missing an explicit `id=...`
- `RUN --mount=type=cache,target=/root/.cache/go-build` was missing an explicit
  `id=...`
- Docker `VOLUME ["/data"]` is unsupported by Railway's builder; Railway
  Volumes must provide `/data`

Repo fix:

- Added `id=go-mod` and `id=go-build` to BuildKit cache mounts.
- Removed Docker `VOLUME ["/data"]`.
- Kept the Railway Volume contract in `railway.toml` with
  `requiredMountPath = "/data"`.

Local validation after the fix:

```sh
docker build --check .
docker build -t kotoba-line:ssh-railway-fix .
go test ./...
```

## Post-Fix Railway Deploy Attempts

Deployment `e5c524a5-242f-4c51-9c51-fb0ba1c3be1f` was created from the patched
working tree with `railway up`.

Deployment `dc554d2c-3b55-4c3c-a833-6b3351d0cbd9` was created with attached
verbose logs:

```sh
railway up . \
  --path-as-root \
  --service kotoba-line-ssh \
  --environment production \
  --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 \
  --ci \
  --verbose \
  --message "Wave 004 verbose Dockerfile fix 2e121b7"
```

Visible build logs only showed:

```text
scheduling build on Metal builder "builder-nenmqy"
```

Railway Agent diagnosis for deployment
`dc554d2c-3b55-4c3c-a833-6b3351d0cbd9`:

- Failing stage: `BUILD_IMAGE`
- Exact blocker: Railway cannot fetch source code from
  `superposition/kotoba-line` because the Railway GitHub App lacks access to
  that repository.
- The build fails at source fetch, before Dockerfile parsing or compilation.

## Current Blocker

Hosted SSH is still blocked outside the repo:

1. Grant the Railway GitHub App access to `superposition/kotoba-line` in GitHub
   app settings, or reconnect the repo from Railway service Settings -> Source.
2. Ensure the service source is `superposition/kotoba-line`, branch `main`, repo
   root.
3. Ensure TCP Proxy exists for internal port `2222`; normal CLI status still
   does not expose a proxy host/port.
4. Trigger a new deploy.
5. Verify:

   ```sh
   ssh player@<tcp-proxy-domain> -p <tcp-proxy-port>
   ```

## Status

Repo-side Dockerfile issues are fixed and locally verified. Railway hosted SSH
cannot be called done until GitHub App access, TCP Proxy host/port, login, and
volume persistence are verified against the live Railway service.
