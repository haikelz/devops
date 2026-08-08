# ArgoCD

## Applications

| App | Namespace | Source Repo |
|---|---|---|
| ai-assistant | default | github.com/haikelz/ai-assistant (k8s/) |

## ai-assistant

Kubernetes manifests synced from `k8s/` directory in the main repo:

| Resource | Purpose |
|---|---|
| `deployment.yaml` | picoclaw AI agent + init containers (workspace seeding, paycheck cron) |
| `pvc.yaml` | Persistent storage for workspace, config, and SQLite DB |
| `services.yaml` | ClusterIP service for picoclaw gateway |
| `network-policy.yaml` | Restrict ingress/egress |
| `argocd-application.yaml` | Self-referencing ArgoCD Application manifest |

### Deployment flow

1. Build image: `docker build -t haikelilham/ai-assistant .`
2. Push: `docker push haikelilham/ai-assistant`
3. Sync: ArgoCD detects image change and rolls out

Or use `../deploy-k8s.sh` which runs `envsubst` + `kubectl apply` directly.

### Init containers

- **initialize-workspace**: seeds `config.json`, `workspace/` (skills + SOUL.md), and `.security.yml`
- **initialize-paycheck-reminder**: registers monthly paycheck cron if not exists

### Services inside the pod

| Service | Port | Binary |
|---|---|---|
| picoclaw | 18790 | `/entrypoint.sh` (upstream) |
| finance-api | 8080 | `/usr/local/bin/finance-api` |
| loker-api | 8081 | `/usr/local/bin/loker-api` |
| loker-bot | — | `/usr/local/bin/loker-bot.sh` (background) |
