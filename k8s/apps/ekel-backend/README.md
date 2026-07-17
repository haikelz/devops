# ekel-backend

Go HTTP API for Wakatime stats, IHSG market data, guestbook, and reactions.

## Runtime

- Go 1.25
- Echo HTTP server
- GORM
- Turso/libSQL in production
- Swagger docs generated under `docs/`

The app listens on `PORT`, defaulting to `9090`. Kubernetes probes use:

```text
GET /ping
```

## Required Environment

For Kubernetes, set these in `k8s/apps/ekel-backend/.env`:

```env
DOMAIN=example.com
IMAGE=registry.example.com/ekel-backend:latest
WAKATIME_API_URL=https://wakatime.com/api/v1/users/current/stats
WAKATIME_API_KEY=
TURSO_DATABASE_URL=
TURSO_AUTH_TOKEN=
IHSG_API_URL=
SECRET_KEY_ADMIN=
SECRET_KEY_CUSTOMER=
ADMIN_PASSWORD=
ADMIN_EMAIL=
JWT_SECRET=
```

Generate JWT signing secrets with:

```bash
openssl rand -base64 64
openssl rand -base64 64
openssl rand -base64 64
```

Use different values for `SECRET_KEY_ADMIN`, `SECRET_KEY_CUSTOMER`, and `JWT_SECRET`.

## Docker

Build from the devops repo root:

```bash
docker build -t ekel-backend-check k8s/apps/ekel-backend
```

The Dockerfile copies `docs/` because `cmd/app/development/main.go` imports `app/docs` for Swagger registration.

## Kubernetes Deploy

From the devops repo root:

```bash
./k8s/deploy-k8s.sh
```

Select `ekel-backend` from the menu.

The deploy script renders:

1. `k8s/k8s/shared/clusterissuer.yaml`
2. `k8s/k8s/ekel-backend/secret.yaml`
3. `k8s/k8s/ekel-backend/services.yaml`
4. `k8s/k8s/ekel-backend/deployment.yaml`
5. `k8s/k8s/ekel-backend/ingress.yaml`

Production must provide `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`. If they are empty, the app falls back to local SQLite, which is not valid for the current `CGO_ENABLED=0` Docker build.
