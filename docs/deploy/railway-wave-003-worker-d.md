# Railway Wave 003 Worker D Attempt

Date: 2026-06-22

## Scope

Issue #10 hosted SSH deployment unblock from current `origin/main`.

No secrets were read, printed, changed, or documented. In particular,
`KOTOBA_SSH_PASSWORD` was not queried.

## Current Main

- Repo: `https://github.com/superposition/kotoba-line`
- Branch used for upload attempt: `origin/main`
- Commit: `683e0dd811eaa3fc708c696398ebf8349afc17ca`
- Commit subject: `Add playable stateful drill wave`

The deploy upload used a temporary detached worktree at that commit so unrelated
local files were not included.

## Railway CLI Capability Check

- Installed CLI: `railway 4.59.0`
- Logged-in account: `Eric Manganaro (ericmanganaro@gmail.com)`
- Relevant visible commands:
  - `railway add --repo ... --service ...`
  - `railway up [PATH] --path-as-root ...`
  - `railway deployment list`
  - `railway logs`
  - `railway service list`
  - `railway volume list`
  - `railway ssh`
  - `railway domain`
- TCP Proxy command status: no CLI command for creating or listing TCP Proxy was
  exposed by `railway --help`, `railway service --help`, or `railway help |
  rg -i "tcp|proxy|domain"`.
- `railway domain` is for Railway/custom HTTP domains, not the raw TCP Proxy
  needed for SSH.

## Resources and State

- Project: `kotoba-line`
  - ID: `b3d1fa80-9e54-4ec2-a88d-420bec09b3b7`
  - Workspace: `Eric Manganaro's Projects`
- Environment: `production`
  - ID: `88b447fa-9608-4c30-8f27-137819fa4311`
- Service: `kotoba-line-ssh`
  - ID: `1e700d31-3245-49d5-9087-f7d279198f52`
  - Service instance ID: `0a1d690e-6bf8-4055-8193-7f1d7527df51`
  - Source: `null`
  - Domains: none
  - Latest deployment: `1979b8d1-cd88-4261-af5d-8b7a3c9a7b85`
  - Latest deployment status: `FAILED`
  - Replicas configured: `1` in `us-west2`
  - Replicas running: `0`
- Volume: `kotoba-line-ssh-volume`
  - ID: `025234ad-b46b-4461-b7f4-321970a1d7d5`
  - Mount path: `/data`
  - Size: `50000` MB
  - Current size: `0.0` MB
  - State: `READY`

## Commands Run

```sh
railway --version
railway --help
railway whoami
railway status --json
railway service list --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 --environment production --json
railway volume --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 --environment 88b447fa-9608-4c30-8f27-137819fa4311 list --json
railway deployment list --service kotoba-line-ssh --environment production --json
railway ssh --help
railway service --help
railway deployment --help
railway domain --help
railway add --help
railway up --help
railway deployment list --help
railway volume --help
railway agent --help
railway service status --help
railway logs --help
railway help | rg -i "tcp|proxy|service domain|domain"
```

Source connection retry:

```sh
railway add --repo superposition/kotoba-line --service kotoba-line-ssh --json
```

Result:

```text
Unauthorized. Please run `railway login` again.
```

Clean main upload attempt:

```sh
git fetch origin main
git rev-parse origin/main
git worktree add --detach /tmp/kotoba-line-origin-main-deploy origin/main
railway up /tmp/kotoba-line-origin-main-deploy \
  --path-as-root \
  --service kotoba-line-ssh \
  --environment production \
  --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 \
  --detach \
  --json \
  --message "Wave 003 deploy origin/main 683e0dd"
railway deployment list --service kotoba-line-ssh --environment production --json --limit 5
railway logs 1979b8d1-cd88-4261-af5d-8b7a3c9a7b85 --service kotoba-line-ssh --environment production --build --lines 500 --json
railway logs 1979b8d1-cd88-4261-af5d-8b7a3c9a7b85 --service kotoba-line-ssh --environment production --deployment --lines 100 --json
railway agent -p "Why did deployment 1979b8d1-cd88-4261-af5d-8b7a3c9a7b85 for service kotoba-line-ssh fail? Please include the exact failing stage and whether repo/source metadata is missing." --service kotoba-line-ssh --environment production --json
```

Upload result:

```json
{
  "deploymentId": "1979b8d1-cd88-4261-af5d-8b7a3c9a7b85",
  "logsUrl": "https://railway.com/project/b3d1fa80-9e54-4ec2-a88d-420bec09b3b7/service/1e700d31-3245-49d5-9087-f7d279198f52?id=1979b8d1-cd88-4261-af5d-8b7a3c9a7b85&"
}
```

Build logs only showed scheduling on a Metal builder and no Docker build output.
Railway Agent diagnosed the failure as:

- Failing stage: `BUILD_IMAGE`
- Missing source metadata:
  - `repo: null`
  - `branch: null`
  - `commitHash: null`
  - `sourceImage: null`
- Root cause: the service still has no connected GitHub repository or Docker
  image source, so Railway has no build source even for the CLI upload.

## Hosted SSH Status

No hosted SSH host or port was achieved.

Reasons:

- GitHub source connection is still blocked by Railway authorization.
- Current CLI upload creates a deployment record but fails before Docker build
  because service source metadata remains null.
- No TCP Proxy exists on the service.
- No deployment is running, so there is no active instance to smoke through
  Railway TCP Proxy or `railway ssh`.

## Dashboard Click Path Required

Official Railway docs currently describe both required remaining actions as
dashboard flows.

Source connection:

1. Open Railway dashboard.
2. Open project `kotoba-line`
   (`b3d1fa80-9e54-4ec2-a88d-420bec09b3b7`).
3. Open service `kotoba-line-ssh`
   (`1e700d31-3245-49d5-9087-f7d279198f52`).
4. Go to `Settings`.
5. In the service source area, choose `Connect Repo`.
6. Select `superposition/kotoba-line`.
7. Set branch/autodeploy branch to `main`.
8. Keep root directory at repo root so Railway reads `/railway.toml`.
9. Deploy after the source is connected.

TCP Proxy:

1. Open the same service `kotoba-line-ssh`.
2. Go to `Settings`.
3. Find `Networking`.
4. Click `TCP Proxy`.
5. Enter internal port `2222`.
6. Save/generate the proxy.
7. Record the generated domain and port.
8. Verify:

   ```sh
   ssh player@<proxy-domain> -p <proxy-port>
   ```

The expected Railway-provided TCP values are also exposed to the running service
as `RAILWAY_TCP_PROXY_DOMAIN`, `RAILWAY_TCP_PROXY_PORT`, and
`RAILWAY_TCP_APPLICATION_PORT` after TCP Proxy exists.

References:

- Railway TCP Proxy docs: <https://docs.railway.com/networking/tcp-proxy>
- Railway Services docs: <https://docs.railway.com/services>
- Railway Variables reference:
  <https://docs.railway.com/variables/reference>

## Blocker

Hosted SSH remains blocked outside the repo:

- Railway GitHub authorization must be refreshed or the source must be connected
  in the dashboard.
- TCP Proxy must be created in the dashboard for internal port `2222`.

No code or Docker config change was indicated by the failed deploy; Railway did
not reach the Docker build step.
