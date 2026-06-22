# Railway Wave 002 Worker C Attempt

Date: 2026-06-22

## Resources Created

- Project: `kotoba-line`
  - ID: `b3d1fa80-9e54-4ec2-a88d-420bec09b3b7`
  - Workspace: `Eric Manganaro's Projects`
  - Environment: `production`
  - Environment ID: `88b447fa-9608-4c30-8f27-137819fa4311`
- Service: `kotoba-line-ssh`
  - ID: `1e700d31-3245-49d5-9087-f7d279198f52`
  - Source: not connected
  - Latest deployment: `87743ddc-50e7-49a1-bfd2-da86d4e81d1d`
  - Latest deployment status: `FAILED`
- Volume: `kotoba-line-ssh-volume`
  - ID: `025234ad-b46b-4461-b7f4-321970a1d7d5`
  - Mount path: `/data`
  - State: `READY`
- Replica config: `us-west2=1`

## Variables Configured

The service has these runtime variables configured:

- `PORT=2222`
- `KOTOBA_SSH_HOST=0.0.0.0`
- `KOTOBA_SSH_HOST_KEY_PATH=/data/ssh_host_ed25519`
- `KOTOBA_STATE_DIR=/data`
- `KOTOBA_SSH_PASSWORD` set to a generated non-default value

Do not commit, publish, or echo the generated password. Reset it from Railway if
it needs to be shared through a secure channel.

## Commands Run

```sh
railway init --name kotoba-line --workspace "Eric Manganaro's Projects" --json
railway add --repo superposition/kotoba-line --service kotoba-line-ssh --json
railway add --service kotoba-line-ssh --json
railway variable set PORT=2222 KOTOBA_SSH_HOST=0.0.0.0 KOTOBA_SSH_HOST_KEY_PATH=/data/ssh_host_ed25519 KOTOBA_STATE_DIR=/data --service kotoba-line-ssh --skip-deploys --json
openssl rand -base64 36 | tr -d '\n' | railway variable set KOTOBA_SSH_PASSWORD --stdin --service kotoba-line-ssh --skip-deploys --json
railway volume --service kotoba-line-ssh add --mount-path /data --json
railway volume --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 --environment 88b447fa-9608-4c30-8f27-137819fa4311 --service 1e700d31-3245-49d5-9087-f7d279198f52 add --mount-path /data --json
railway up --service kotoba-line-ssh --environment production --detach --json --message "Deploy SSH skeleton commit 2a080d8"
railway up --service kotoba-line-ssh --environment production --ci --json --message "Deploy clean main commit 2a080d8"
railway up --service kotoba-line-ssh --environment production --ci --verbose --message "Verbose clean main deploy 2a080d8"
railway service scale --service kotoba-line-ssh --environment production us-west=1 --json
railway service list --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 --environment production --json
railway volume --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 --environment 88b447fa-9608-4c30-8f27-137819fa4311 list --json
```

## Blockers

- GitHub deploy source connection is blocked by Railway/GitHub authorization.
  `railway add --repo superposition/kotoba-line --service kotoba-line-ssh`
  returned `Unauthorized. Please run railway login again.` while
  `railway whoami` still reports the CLI is logged in as
  `Eric Manganaro (ericmanganaro@gmail.com)`.
- CLI upload deploys from a clean `origin/main` worktree uploaded source bytes
  but still failed before Docker build logs. Railway's built-in agent diagnosed
  deployment `87743ddc-50e7-49a1-bfd2-da86d4e81d1d` as `config_error` at
  `BUILD_IMAGE`: the service has `repo: null`, `branch: null`,
  `commitHash: null`, and `sourceImage: null`.
- TCP Proxy could not be enabled with the installed Railway CLI. `railway
  --help` exposes `domain` for HTTP domains but no TCP Proxy command. Railway's
  TCP Proxy documentation describes the setup as a service Settings dashboard
  flow: create TCP Proxy and enter internal port `2222`.
- Hosted SSH was not verified because no TCP Proxy host/port was created and
  the service has no running deployment.

## Current Verification

```sh
railway service list --project b3d1fa80-9e54-4ec2-a88d-420bec09b3b7 --environment production --json
```

Current service state:

- `source: null`
- `status: FAILED`
- `url: null`
- `replicas.configured: 1`
- `replicas.running: 0`
- volume `/data`: `READY`
