# NOTE

All of the guide .md in this repo is AI-GENERATED and used for me to deep dive into Devops.

## What's inside

### Infrastructure-as-Code (Terraform)

GCP provisioning with HashiCorp Terraform:

- **Storage**: `google_storage_bucket` resource for object storage
- **Kubernetes**: GKE cluster provisioning via `google_container_cluster` and node pools
- **State management**: local `terraform.tfstate` tracking

Familiarises with: providers, resources, variables, outputs, state files, and the `plan`/`apply` workflow.

### Container Orchestration (Kubernetes)

A monorepo (`k8s/`) with commitlint-conventional commits, lefthook git hooks, and knip dead-code analysis. Four real applications deployed via Kubernetes manifests:

| App | Stack | Description |
|---|---|---|
| **mazanoke** | nginx static site | Simple web deployment with ingress + TLS via cert-manager/LetsEncrypt |
| **mbakmegumi** | Astro 6 + React 19 + GSAP + Three.js | Interactive frontend served through nginx, with Docker multi-stage builds |
| **ryuko-matoi-go** | Go + whatsmeow + SQLite + AI APIs | WhatsApp bot with finance tracking, scheduling, OCR, and multi-platform posting (Twitter, Instagram, TikTok, Facebook) |
| **umami** | Node.js + PostgreSQL 16 | Privacy-focused web analytics with StatefulSet for persistent database storage |

All apps follow a consistent K8s pattern: **Deployment** with security-hardened containers (non-root, read-only rootfs, dropped capabilities), **ClusterIP Service**, **Ingress** with cert-manager TLS, **topology spread constraints**, and startup/readiness/liveness probes.

Umami adds: init container for dependency ordering, StatefulSet with PVCs, NetworkPolicy for pod-level segmentation.

A shared `ClusterIssuer` provisions LetsEncrypt certificates via HTTP-01 challenge (Traefik ingress controller).

### Deploy Scripts

Each app has a `deploy-k8s.sh` that:

1. Sources `.env` for secrets
2. Validates required variables
3. Renders K8s YAML templates via `envsubst`
4. Applies in dependency order: `clusterissuer → secret → services → deployment → ingress`
5. Refuses to apply unsubstituted `${VARS}`

Zero manual `kubectl` commands — one script deploys each app end-to-end.

## Prerequisites

- Docker (assumed familiar)
- A Kubernetes cluster with **Traefik** ingress controller and **cert-manager** installed
- GCP project with billing enabled (for Terraform)
- `kubectl`, `terraform`, `envsubst` on PATH

## Quick start

```bash
# 1. Bootstrap GCP infrastructure
cd terraform
terraform init
terraform plan
terraform apply

# 2. Deploy an application
cd k8s/apps/<app-name>
cp .env.example .env
# edit .env with your values
./deploy-k8s.sh
```

## Repository layout

```
devops/
├── terraform/              # GCP infrastructure (Terraform)
│   ├── main.tf             # Provider + bucket resource
│   ├── gke.tf              # GKE cluster definition
│   ├── variables.tf        # Input variables
│   └── output.tf           # Stack outputs
├── k8s/                    # Kubernetes orchestration monorepo
│   ├── apps/               # Application source + Dockerfiles
│   │   ├── mazanoke/       # Static nginx site
│   │   ├── mbakmegumi/     # Astro + React frontend
│   │   ├── ryuko-matoi-go/ # Go WhatsApp bot
│   │   └── umami/          # Umami analytics
│   ├── k8s/                # Kubernetes manifests
│   │   ├── shared/         # Shared resources (ClusterIssuer)
│   │   └── {app}/          # Per-app: deployment, service, ingress, secret
│   ├── package.json
│   ├── commitlint.config.js
│   ├── lefthook.yml
│   └── knip.json
├── .gitignore
├── .opencodeignore
└── README.md
```

## Topics covered

- **Terraform**: providers, resources, variables, state, GCP
- **Kubernetes**: Deployments, Services, Ingresses, StatefulSets, ConfigMaps/Secrets, NetworkPolicies, probes, security contexts, topology spread, cert-manager
- **Docker**: multi-stage builds, Compose for local dev, image tagging
- **Git hooks**: commitlint for conventional commits, lefthook for pre-push checks
- **CI/CD**: `.gitlab-ci.yml` for the Go bot
- **Security**: non-root containers, read-only filesystems, dropped capabilities, seccomp profiles, network segmentation
- **Monitoring**: startup/readiness/liveness probes

## Full DevOps Learning Guide

For a complete from-zero-to-job-ready curriculum covering Linux, Git, Terraform,
Kubernetes, Helm, Ansible, CI/CD (GitHub Actions + ArgoCD), cloud platforms,
monitoring, security, portfolio projects, and interview prep, see:

**[DEVOPS-GUIDE.md](DEVOPS-GUIDE.md)** — structured in 10 phases with weekly
study plans, concrete "do this" exercises, and recommended resources.

A standalone PDF version is also available: **`DEVOPS-GUIDE.pdf`**.

## Learning path

If you're following along:

1. Start with **Terraform** — provision a GKE cluster and a storage bucket
2. Get comfortable writing **Kubernetes manifests** — deployments and services
3. Add **ingress** with cert-manager for TLS termination
4. Progress to **StatefulSets** for stateful workloads (PostgreSQL)
5. Harden with **security contexts**, **NetworkPolicies**, and **topology spread constraints**
6. Wire up **git hooks** and **commit linting** for team consistency
7. Deploy real applications — from static sites to AI-integrated Go bots

## License

Private learning project.
