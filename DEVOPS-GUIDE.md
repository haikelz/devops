# DevOps Learning Guide — SWE to DevOps Engineer

A complete, structured curriculum for a **fullstack software engineer** (JavaScript/TypeScript/Go, Docker, basic K8s) transitioning to DevOps Engineer. You already know how to build software — this guide teaches you how to build, deploy, and operate the platforms that run it.

---

## SWE to DevOps: Your Bridge Map

DevOps is not a new career. It is applying your existing SWE skills to a different layer of the stack.

### What You Already Know (That Translates Directly)

| SWE Skill | DevOps Translation |
|---|---|
| **Programming** (JS/TS/Go) | IaC (Terraform, Ansible), automation scripts, CI/CD pipeline DSLs |
| **Debugging** (breakpoints, logs, traces) | Infrastructure troubleshooting (kubectl describe, terraform plan, Prometheus metrics) |
| **API design** (REST, gRPC, contracts) | Service interfaces (ClusterIP, Ingress), cloud API design, infra module contracts |
| **Async thinking** (promises, goroutines) | Distributed systems reasoning, eventual consistency, message queues |
| **Version control** (Git, PRs, branching) | GitOps (ArgoCD, Flux), configuration-as-code versioning, state management |
| **Testing** (unit, integration, e2e) | Infrastructure testing (terratest, plan validation, chaos engineering) |
| **Code review** (patterns, bugs) | Infrastructure review (blast radius analysis, security groups, cost impact) |

### What is Genuinely New

- **Networking fundamentals** — You have used `fetch()` and HTTP clients. Now you need subnets, CIDR blocks, route tables, load balancers, TLS termination at the infrastructure level, and how packets traverse VPCs.
- **Infrastructure lifecycle** — Servers are not deployed once and forgotten. They need provisioning, scaling, patching, monitoring, and decommissioning. Entire lifecycles managed through code.
- **State management** — Terraform state, Kubernetes etcd, configuration drift detection. Infrastructure state IS the source of truth — not the running environment.
- **Failure modes** — Your app crashes with a stack trace. Infrastructure fails silently, degrades gradually, or takes down your cluster. Think blast radius and graceful degradation.
- **On-call mindset** — You are keeping systems alive. Alert fatigue, incident response, post-mortems, error budgets — these are core job functions.
- **Cloud economics** — Every resource costs money, every hour. From free localhost to ~$70/month for a dev cluster. Cost awareness becomes second nature.

### What You Need to Unlearn

| SWE Habit | DevOps Reality |
|---|---|
| "Works on my machine" | Your machine does not matter. It works in production or it does not work. |
| SSH into the server to fix it | Servers are cattle, not pets. If one breaks, kill it and a new one spawns. Never SSH. |
| Deploy by pushing code | Deploy by merging config. GitOps means repo state IS cluster state. |
| Focus on app code | The app is 20%. Networking, storage, security, scaling, monitoring — the other 80%. |
| Manual one-off fixes | Everything automated, idempotent, repeatable. Do it manually once, you will do it wrong the second time. |
| "I'll set up the DB later" | Infrastructure comes first. No app deploys without somewhere to run. |

### The Mindset Shift

As a SWE, you built features users interact with. As a DevOps engineer, you build **platforms that run features reliably.**

Your users are other developers. Your feature is their deployment pipeline. Your UX is `kubectl apply`, your API is a Terraform module, your error messages are 500 pages during an outage.

Success means: developers ship faster, deployments do not fail, and nobody pages you at 3 AM.

---

## How to use this guide

Each **phase** represents a learning milestone. Within each phase, **topics** are ordered by dependency — complete them in sequence. Every topic has a **do this** section with concrete actions. Don't just read; type every command, break every config, fix it, then move on.

**Estimated timeline**: 6–12 months of consistent part-time study (10–15 hrs/week).

---

## Phase 0: The Prerequisites

Before touching any DevOps tool, you need actual computer fundamentals. These are the filters that screen out most candidates.

### 0.1 Linux Fundamentals

Know your way around a headless server. No GUI, no mouse.

- Filesystem hierarchy (`/`, `/etc`, `/var`, `/proc`, `/sys`, `/tmp`)
- File operations: `ls`, `cp`, `mv`, `rm`, `find`, `locate`
- Text processing: `grep`, `sed`, `awk`, `cut`, `sort`, `uniq`, `wc`
- Permissions: `chmod`, `chown`, `umask`, special bits (SUID, SGID, sticky)
- Process management: `ps`, `top`/`htop`, `kill`, `systemd` units
- Package management: `apt`/`yum`/`apk`
- Networking: `ip`, `ss`, `ping`, `traceroute`, `dig`, `curl`
- Systemd: writing service units, journalctl, timers
- Filesystem management: `df`, `du`, `mount`, `fstab`, LVM basics

**Do this**: Install Ubuntu Server in a VM (or use a cheap VPS). Break it. Fix it. Repeat until you can recover a system where you accidentally removed `libc6`.

### 0.2 Bash Scripting

The glue of everything DevOps.

- Variables, arrays, parameter expansion
- Conditionals (`if`, `case`) and loops (`for`, `while`)
- Functions and scope
- Exit codes and error handling (`set -euo pipefail`)
- Argument parsing (`$@`, `getopts`)
- Subshells, command substitution, process substitution
- `jq` for JSON manipulation

**Do this**: Write a backup script that compresses a directory, uploads to a remote server, and cleans up files older than 7 days. Include proper error handling and logging.

### 0.3 Git

Not just `add`/`commit`/`push`. Understand the model.

- Object model: blobs, trees, commits, tags
- Branching and merging strategies (Git Flow, trunk-based)
- Rebasing vs merging — when each applies
- Interactive rebase (`rebase -i`) — squash, reword, drop
- Cherry-pick, revert, reset (soft/mixed/hard)
- `bisect` for finding the commit that broke something
- Hooks (pre-commit, pre-push)
- Conventional commits

**Do this**: Intentionally create a messy branch with 10+ commits. Use interactive rebase to squash, reword, and reorder them into 3 clean commits. Then use `git bisect` to find where a bug was introduced.

### 0.4 Networking Fundamentals

You cannot manage infrastructure without understanding how data moves.

- OSI model (focus on layers 3, 4, 7)
- TCP vs UDP — three-way handshake, connection states
- DNS — record types (A, AAAA, CNAME, MX, TXT), resolution flow
- HTTP/HTTPS — methods, status codes, headers, cookies, caching
- TLS — handshake, certificates, CAs, SNI
- Load balancing concepts — round-robin, least connections, sticky sessions
- Subnetting — CIDR notation, public vs private IPs, NAT

**Do this**: Use `tcpdump` or `tshark` to capture a TLS handshake, then identify each step (ClientHello, ServerHello, Certificate, etc.) from the capture.

---

## Phase 0.5: The DevOps Mindset (SWE Bridge)

Before diving into tools, internalize the operating philosophy. This separates a "developer who runs Docker" from a DevOps engineer.

### 12-Factor App Principles (Through DevOps Lens)

You have likely read [12factor.net](https://12factor.net). Here is the DevOps perspective:

1. **Codebase**: One app = one repo. Terraform repo: same rule. One repo per infrastructure module.
2. **Dependencies**: Declare explicitly. Terraform provider versions pinned (NEVER `latest`), Docker base image tags pinned to SHA digests.
3. **Config**: Store in environment, not code. Kubernetes Secrets, Terraform variables, GitHub Actions secrets — same principle, different layer.
4. **Backing services**: Treat as attached resources. Database URL changes per environment — your app should not care. K8s Services have stable DNS names for this reason.
5. **Build, release, run**: Strict separation. Build a Docker image once, tag it, deploy the same artifact to dev/staging/prod. Never rebuild between environments.
6. **Processes**: Stateless, share-nothing. State goes in a database or object store. Pods are ephemeral; StatefulSets exist for stateful workloads.
7. **Port binding**: Export services via port binding. App listens on `:3000`, Service maps to port 80, Ingress exposes externally. Each layer explicit.
8. **Concurrency**: Scale out via the process model. Kubernetes replicas, AWS Auto Scaling Groups, Lambda concurrency — horizontal scaling.
9. **Disposability**: Fast startup, graceful shutdown. SIGTERM → drain connections → exit 0. K8s gives your pod 30 seconds before SIGKILL. Respect it.
10. **Dev/prod parity**: Keep environments similar. Docker helps; same cloud provider, database engine, resource sizes help more.
11. **Logs**: Treat as event streams. App writes to stdout. Fluentd/Promtail picks it up. Loki/Elasticsearch indexes it. Grafana/Kibana displays it. Never SSH and `tail -f`.
12. **Admin processes**: Run as one-off. `kubectl exec` for debugging only. `terraform taint` for state manipulation. Everything scripted.

### Cattle, Not Pets

**SWE analogy**: You have 100 API instances behind a load balancer. One starts returning 500s. Do you SSH in and debug? No — kill it. The load balancer routes to healthy instances. A new one spins up from the same Docker image.

"Cattle, not pets" — infrastructure is disposable, replaceable, uniform. No emotional attachment to servers. No manual patching. Redeploy from a golden image.

**Example from this repo**: Every app in `k8s/` uses a Deployment with `replicas` and `strategy: RollingUpdate`. If a pod crashes, K8s replaces it. If a node dies, pods reschedule. The Deployment spec is the truth — running pods are cattle.

### Immutable Infrastructure

SWEs deploy mutably: SSH in, `git pull`, `npm install`, restart. This works once — then configuration drift sets in.

Immutable infrastructure: never modify a running server. Build a new image, deploy it, kill the old one. Every deploy is a clean slate from a known good state. Docker makes this natural — your Dockerfile IS the server configuration.

### Idempotency

As a SWE, you write idempotent APIs: `PUT /users/42` with the same payload always produces the same result. Apply this to infrastructure:

- `kubectl apply -f deployment.yaml` → applied twice = same state
- `terraform apply` → runs twice = no changes on second run
- `ansible-playbook site.yml` → runs twice = all tasks report "ok"

`terraform plan` shows the diff between desired state (`.tf` files) and actual state (`.tfstate` file). It is a dry-run for infrastructure — imagine `git push` showing a diff before applying.

---

## Phase 1: Version Control & Collaboration

Master the Git workflows used in real teams.

### 1.1 Git Platform Proficiency

Pick one platform (GitHub recommended for market share).

- PR/MR workflows — code review, approvals, draft PRs
- Branch protection rules — required checks, status gates
- Actions basics — YAML syntax, workflow triggers, runners
- Repository management — webhooks, deploy keys, secrets
- GitHub CLI (`gh`) for terminal-based workflow

**Do this**: Set up a repo with branch protection requiring PRs, passing status checks, and linear history. Automate linting in CI.

---

## Phase 2: Infrastructure as Code (IaC)

Your config files are now the source of truth.

### 2.1 Terraform

The dominant IaC tool. HashiCorp Terraform (or OpenTofu).

- Core concepts: desired state, execution plan, resource graph
- HCL syntax — blocks, arguments, expressions, functions
- Providers and resources
- State management — local vs remote (S3/GCS + DynamoDB/BigQuery locking)
- Variables and outputs
- Modules — writing, publishing, versioning
- Workspaces for environment separation
- `terraform workspace`, `import`, `refresh`, `state` subcommands
- Remote backends (S3, GCS, AzureRM, Terraform Cloud)
- Sentinel / OPA policy as code (intro)

**Do this**: Write Terraform that provisions a VPC, a subnet, an EC2 instance with a security group, and an S3 bucket. Use a module from the registry. Configure remote state.

**SWE Connection** — Terraform's HCL is a declarative DSL. You describe WHAT you want, not HOW to create it. Like React (`<div className="card">`) vs imperative DOM (`document.createElement`). `terraform plan` is like TypeScript's type checker — it tells you what will happen BEFORE it happens. Go developers: `terraform plan` returns a diff. If wrong, you do not apply. Same pattern as `if err != nil { return err }` — check before proceeding.

**SWE Note on Pulumi**: As a TypeScript/Go developer, Pulumi will feel natural — write infra in your language with real loops, functions, types. Terraform's HCL will feel primitive. Learn Terraform first anyway — 80%+ of the market uses it. Concepts transfer; syntax is the minor part.

**Common SWE Pitfalls**:
1. **Manual console changes** — Add a tag in the AWS console, then `terraform plan` shows a diff wanting to remove it. Terraform is source of truth; the console is read-only. Fix: `terraform import` the resource, or delete the manual change.
2. **State file corruption** — `.tfstate` maps HCL to real cloud resources. Never edit manually. Never lose it. Use remote state with versioning (S3 + DynamoDB locking).
3. **"I'll just terraform destroy and rebuild"** — Works in dev. In production, destroy deletes your database, DNS, load balancer. Blast radius = entire cloud account. In production, `terraform apply -target` for surgical changes.

**Troubleshooting FAQ**:
- `Error: Error acquiring the state lock` → Someone else is running apply. Wait, or `terraform force-unlock` (dangerous).
- `Error: Provider produced inconsistent result after apply` → Cloud provider returned unexpected result. Run `terraform refresh` then re-apply.
- `Error: Error creating S3 bucket: BucketAlreadyExists` → Bucket names are globally unique across ALL AWS accounts.

**Cost Note**: The VPC + EC2 exercise costs ~$5-10/month if left running. ALWAYS `terraform destroy` after exercises. Set an AWS Budget Alert before provisioning anything.

**Repo Reference**: This repo has a GCP GKE Terraform root at `terraform/`. Read `main.tf` for the provider, VPC, and Cloud NAT; `gke.tf` for the regional GKE cluster and node pool; and `variables.tf` for parameterization.

### 2.2 Infrastructure Testing

- `terraform plan` in CI with review
- `terraform-compliance` or `terraform-sentinel` for policy checks
- `terratest` for integration tests
- `checkov` / `tfsec` / `trivy` for security scanning

### 2.3 Beyond Terraform: Alternatives

- **Pulumi** — same concept, general-purpose languages (TypeScript, Python, Go)
- **Crossplane** — Kubernetes-native IaC, control plane pattern
- **CDK for Terraform** — Terraform via programming languages

Know about them. Deep-dive one if your target job uses it.

---

## Phase 3: Container Orchestration — Kubernetes

K8s is the non-negotiable skill for DevOps roles.

### 3.1 Core Concepts

- Architecture: control plane (API server, scheduler, controller manager, etcd) vs worker nodes (kubelet, kube-proxy, container runtime)
- Pods — the atomic unit
- Workloads: Deployments, StatefulSets, DaemonSets, Jobs, CronJobs
- Services: ClusterIP, NodePort, LoadBalancer, ExternalName
- Ingress and ingress controllers
- ConfigMaps and Secrets
- Namespaces for isolation
- Resource requests and limits (CPU, memory)
- Probes: startup, readiness, liveness

**Do this**: Deploy a simple web app (nginx) using `kubectl run`, then recreate it with YAML manifests. Add a Service, then an Ingress. Scale it. Do a rolling update. Roll back.

**SWE Connection** — Kubernetes is a distributed operating system. Map it to programming concepts:

| K8s Concept | Programming Analogy |
|---|---|
| Pod | OS process — smallest schedulable unit |
| Deployment | Process supervisor (systemd) keeping N replicas running |
| Service | Stable interface/API endpoint for a set of pods |
| Ingress | Reverse proxy (nginx) routing external traffic |
| ConfigMap/Secret | Environment variables + config files, declarative |
| Namespace | Module/package boundary isolating resources |
| Probes | Health check endpoints your supervisor calls |

**The K8s API is just an HTTP API** — `kubectl` is a CLI client. You could call `POST /api/v1/namespaces/default/pods` with `curl`. You are interacting with a REST API that manages distributed compute resources.

**Common SWE Pitfalls**:
1. **"I'll just kubectl exec into the pod and fix it"** — This is patching production directly. The fix disappears when the pod restarts. Fix the manifest, not the running pod.
2. **"My app works locally, why doesn't it in K8s?"** — Resource limits. Your local machine has 16GB RAM. Your pod has 256Mi. Add `resources.limits.memory` or your app gets OOMKilled.
3. **"I'll use NodePort for my service"** — NodePort exposes your app on a high port on every node. Fine for learning; production uses Ingress or LoadBalancer. NodePort + public node IPs = security incident.

**Troubleshooting FAQ**:
- **Pod stuck in `CrashLoopBackOff`** → `kubectl logs <pod> --previous`. 90% of the time: missing env var, wrong command, or app panics on startup.
- **Pod stuck in `Pending`** → `kubectl describe pod <pod>`, check Events. Usually: insufficient CPU/memory, nodeSelector mismatch, or PVC not bound.
- **Service not routing traffic** → `kubectl get endpoints <service>`. If empty, selector labels do not match pod labels. Service selectors use label matching — like a database query with WHERE.

**Cost Note**: A 2-node GKE cluster runs ~$70/month. For learning: use minikube or kind (free, local) for basics, then a cheap managed cluster for Ingress/cert-manager practice. Destroy when not in use.

**Repo Reference**: This repo's `k8s/` directory is a real K8s monorepo. Study: `k8s/mazanoke/` (simplest: deploy + service + ingress + cert-manager), `k8s/goatcounter/` (adds PVC + NetworkPolicy), `k8s/beszel/` (adds DaemonSet for node monitoring).

### 3.2 Storage

- Volumes, PersistentVolumes, PersistentVolumeClaims
- StorageClasses and dynamic provisioning
- StatefulSets with volumeClaimTemplates
- CSI drivers

**Do this**: Deploy PostgreSQL as a StatefulSet with persistent storage. Scale it down and up. Verify data survives pod deletion.

### 3.3 Security

- RBAC — Roles, ClusterRoles, RoleBindings, ClusterRoleBindings, ServiceAccounts
- Pod Security Standards / Pod Security Admission
- NetworkPolicies for pod-level segmentation
- Security contexts — runAsNonRoot, readOnlyRootFilesystem, capabilities
- Seccomp profiles
- Secrets encryption at rest

### 3.4 Package Management (Helm)

- Charts: structure, templates, values
- Built-in objects (Release, Chart, Files, Capabilities)
- Template functions and pipelines
- Dependency management
- Repositories and publishing
- Helmfile for multi-environment releases

**Do this**: Create a Helm chart for the web app you deployed earlier. Parameterize the image tag, replicas, and resource limits.

### 3.5 Service Mesh (optional but differentiator)

- Istio or Linkerd basics
- Sidecar injection, mTLS, traffic splitting
- Observability (metrics, traces, access logs)

**Do this**: Deploy Istio on your cluster, enable mTLS, and route 10% of traffic to a canary version of your app.

---

## Phase 4: Configuration Management & Automation

### 4.1 Ansible

Agentless, push-based, YAML-driven.

- Inventory (static and dynamic)
- Modules: `command`, `copy`, `template`, `service`, `apt`, `file`
- Playbooks — plays, tasks, handlers, variables
- Roles and directory structure
- Jinja2 templating
- Ansible Vault for secrets
- Pull mode vs push mode

**Do this**: Write an Ansible playbook that provisions a web server — install nginx, configure a virtual host, deploy an HTML file, open the firewall.

### 4.2 Other Tools (Know of Them)

- **Chef** / **Puppet** — older, agent-based, still in many enterprises
- **Salt** — fast, event-driven
- **Packer** — machine image as code (AMI, GCE image, Vagrant box)

---

## Phase 5: CI/CD Pipelines

Automate everything from commit to production.

### 5.1 CI/CD Concepts

- Continuous Integration: merge often, build and test every commit
- Continuous Delivery: artifacts always deployable
- Continuous Deployment: every commit that passes tests goes to production
- Pipeline stages: lint → test → build → scan → deploy
- Artifact management (registry, versioning)

### 5.2 GitHub Actions

The easiest to start with and widely used.

- Workflow syntax: `on`, `jobs`, `steps`, `runs-on`
- Actions marketplace — reusable actions
- Self-hosted runners
- Environments, secrets, and approvals
- Matrix builds
- OIDC for cloud auth (no static creds)

**Do this**: Build a CI pipeline that lints, tests, builds a Docker image, pushes to Docker Hub, and deploys to a staging K8s cluster.

### 5.3 GitLab CI

- `.gitlab-ci.yml` — stages, jobs, artifacts, caching
- Runners (shared, specific, group)
- Auto DevOps
- Review Apps (ephemeral environments per MR)

### 5.4 GitOps with ArgoCD

ArgoCD is the industry standard for Kubernetes GitOps.

- Application CRD, Sync policy, sync waves
- SSO integration (dex, OIDC)
- Image updater
- Argo Rollouts for progressive delivery (blue/green, canary)
- Multi-cluster management

**Do this**: Bootstrap an ArgoCD Application that syncs a deployment from a Git repository. Make a change in Git — watch ArgoCD apply it automatically.

### 5.5 Other Tools (Know of Them)

- **Jenkins** — still ubiquitous in enterprise. Groovy pipelines, shared libraries
- **CircleCI**, **Buildkite** — SaaS alternatives
- **Flux** — alternative GitOps operator

---

## Phase 6: Cloud Platforms

Deep-dive on **one** cloud. Know the other two at conversation level.

### 6.1 AWS (largest market share)

- Core: EC2, S3, VPC, IAM, Route53
- Containers: ECS, EKS, ECR
- Databases: RDS, Aurora
- Networking: ALB/NLB, CloudFront, VPN
- Security: KMS, Secrets Manager, WAF, Shield
- Automation: CloudFormation, CDK, SSM
- Serverless: Lambda, API Gateway, EventBridge

### 6.2 GCP

- Core: Compute Engine, Cloud Storage, VPC, IAM, Cloud DNS
- Containers: GKE (best managed K8s experience), Artifact Registry
- Networking: Cloud Load Balancing, Cloud CDN, Cloud NAT
- Security: Cloud KMS, Secret Manager, Cloud Armor
- Automation: Deployment Manager, Cloud Build

### 6.3 Azure

- Core: VMs, Blob Storage, VNet, Entra ID
- Containers: AKS, ACR
- Networking: Azure Load Balancer, Front Door, Application Gateway
- Security: Key Vault, Defender for Cloud

**Do this**: Get a **certification**. This forces structured learning and looks good on a resume.
- AWS: Solutions Architect Associate (SAA-C03)
- GCP: Associate Cloud Engineer
- Azure: AZ-104

---

## Phase 7: Monitoring & Observability

You cannot improve what you don't measure.

### 7.1 Metrics & Alerting

- **Prometheus**: data model, PromQL, exporters (node_exporter, kube-state-metrics), ServiceMonitor, recording rules, alerting rules
- **Grafana**: dashboards, panels, alerting, annotations
- Alertmanager: grouping, inhibition, routing, silences
- **PagerDuty** / **Opsgenie** for on-call

**Do this**: Set up kube-prometheus-stack (Prometheus + Grafana) on your K8s cluster. Create a dashboard showing CPU/memory/disk across all nodes. Set an alert for >80% disk usage that fires to a webhook.

### 7.2 Logging

- **EFK/ELK stack**: Elasticsearch, Fluentd/Logstash, Kibana
- **Loki**: Prometheus-inspired, horizontally scalable, cheap
- **OpenSearch**: AWS fork of Elasticsearch

**Do this**: Deploy Loki + Promtail on your cluster. Feed container logs to Grafana. Create a log-based alert.

### 7.3 Tracing

- **OpenTelemetry** — the unified standard (traces + metrics + logs)
- Jaeger / Tempo for trace storage
- Distributed tracing concepts: spans, trace context, sampling

**Do this**: Instrument a simple Go app with OpenTelemetry, send traces to Jaeger, and visualize a request that spans two services.

### 7.4 SLOs and Error Budgets

- SLI, SLO, SLA definitions
- Error budget policy
- Burn rate alerting
- Multi-window, multi-burn-rate approach

---

## Phase 8: Security (DevSecOps)

Security is not a phase — it's embedded in every phase above. Call it out explicitly here.

### 8.1 Shift-Left Security

- **SAST**: Semgrep, CodeQL, SonarQube — find bugs before commit
- **DAST**: OWASP ZAP, Burp Suite — test running applications
- **SCA**: Trivy, Grype, Snyk — scan dependencies for known CVEs
- **Container scanning**: Trivy, Docker Scout, Anchore
- **IaC scanning**: Checkov, tfsec, Trivy

### 8.2 Supply Chain Security

- **SLSA** framework (Supply-chain Levels for Software Artifacts)
- **SBOM** generation (Syft, Trivy)
- **Sigstore** / **Cosign** for signing containers
- **in-toto** attestations

### 8.3 Secrets Management

- **HashiCorp Vault**: dynamic secrets, transit engine, KV store, K8s auth
- **External Secrets Operator** / **Secrets Store CSI Driver** for K8s
- **Mozilla SOPS** + age/GPG for encrypted files in Git

**Do this**: Deploy Vault on your cluster. Configure Kubernetes auth. Write a controller that reads a DB password from Vault and injects it into pods.

### 8.4 Runtime Security

- **Falco** — behavioral activity monitoring (the K8s security sibling of Sysdig)
- **OPA/Gatekeeper** — admission controller for policy enforcement
- **Kyverno** — Kubernetes-native policy engine

---

## Phase 9: Real Projects (Build a Portfolio)

Theory is worthless without demonstrated work. Build these end-to-end and put them on GitHub.

### Project 1: Automated Deployment Pipeline

- Source code in GitHub
- CI pipeline (GitHub Actions): lint → test → build → scan → publish image
- GitOps deploy (ArgoCD): auto-sync to dev, manual promotion to prod
- Infrastructure (Terraform): VPC, EKS cluster, node groups
- Monitoring: Prometheus metrics, Grafana dashboard, Loki logs
- Result: you commit, CI builds, ArgoCD deploys. End to end.

### Project 2: Multi-Tier Application on K8s

- Frontend: nginx serving React/Vue/Astro static build
- Backend API: Go or Node.js
- Database: PostgreSQL StatefulSet
- Backup: CronJob that pg_dumps to S3/GCS
- Ingress with TLS (cert-manager + LetsEncrypt)
- NetworkPolicy restricting pod-to-pod traffic

### Project 3: Disaster Recovery Simulation

- Terraform provisions infra in two regions
- Application runs in primary region
- Script that simulates primary failure (terraform destroy partial)
- Automate failover to secondary region
- Test RTO and RPO

### Project 4: Kubernetes Operator (advanced)

- Write a custom operator in Go using `controller-runtime`
- Watch a custom CRD
- Reconcile desired state
- Deploy with OLM

---

## Phase 10: Soft Skills & Career

### 10.1 System Design for DevOps

Common interview scenarios:

- Design a CI/CD pipeline for a microservices architecture
- Design a highly available K8s cluster across availability zones
- Design a logging infrastructure for 500 microservices
- Design a multi-region deployment strategy
- Design secrets management for 100+ engineers

Framework: understand requirements → sketch data flow → identify failure modes → propose solution → discuss tradeoffs.

### 10.2 Behavioral Questions

- "Tell me about a time an incident happened and how you handled it"
  — Use STAR: Situation, Task, Action, Result
- "How do you handle on-call and burnout?"
- "Tell me about a time you automated something that was manual"
- "How do you stay up to date with industry changes?"

### 10.3 Resume Tips

- Quantify impact: "Reduced deployment time from 45min to 8min"
- List tools, but more importantly what you achieved with them
- GitHub link with real projects beats "3 years experience" every time
- Include CI/CD badges in your project READMEs

### 10.4 Certifications Path (Optional but Helpful)

| Cert | Provider | When |
|---|---|---|
| Linux Essentials / LPIC-1 | LPI | Before or during Phase 1 |
| CKA (Certified Kubernetes Administrator) | CNCF | After Phase 3 |
| AWS SAA (Solutions Architect Associate) | AWS | During Phase 6 |
| CKAD (Certified Kubernetes App Developer) | CNCF | After CKA |
| Terraform Associate | HashiCorp | After Phase 2.1 |

CKA is the highest-value for DevOps roles. Do not skip it.

### 10.5 Interview Prep Checklist

Before applying, confirm you can:

- [ ] Explain what happens when you type `kubectl create deploy nginx --image=nginx`
  (API call → etcd write → scheduler → kubelet → container runtime → pod running)
- [ ] Debug a pod that's stuck in `CrashLoopBackOff` or `Pending`
- [ ] Write a Dockerfile that produces a small, secure image
- [ ] Write a Terraform config that provisions infrastructure with remote state
- [ ] Set up a CI pipeline that builds, scans, and deploys
- [ ] Explain the difference between a Deployment and a StatefulSet
- [ ] Explain how `terraform plan` works internally
- [ ] Describe how you'd migrate a 500-node cluster from one K8s version to the next without downtime

---

## Recommended Resources

### Books

- **The DevOps Handbook** — Gene Kim et al. (cultural foundation)
- **Site Reliability Engineering** — Google (SRE principles)
- **Kubernetes in Action** — Marko Lukša (best K8s book)
- **Terraform: Up & Running** — Yevgeniy Brikman
- **The Phoenix Project** — Gene Kim (novel, but explains DevOps culture)

### Free Courses

- [KodeKloud](https://learn.kodekloud.com) — hands-on labs, great for K8s
- [Killercoda](https://killercoda.com) — free interactive scenarios
- [Play with Kubernetes](https://labs.play-with-k8s.com) — free K8s sandbox
- [Terraform Up & Running workshop](https://github.com/brikis98/terraform-up-and-running-code)
- [Awesome DevOps](https://github.com/awesome-devops/awesome-devops)

### Certification Prep

- [Killer.sh](https://killer.sh) — CKA/CKAD simulator (best mock exams)
- [ExamPro](https://www.exampro.co) — AWS cert prep
- [Terraform Certification Guide](https://github.com/affinidi/terraform-associate-certification)

### Practice Platforms

- [Killercoda](https://killercoda.com) — scenarios on demand
- [Instruqt](https://instruqt.com) — hands-on tracks
- [Katacoda (archived but great content)](https://www.katacoda.com)
- [Play with Docker](https://labs.play-with-docker.com)

### Communities

- [r/devops](https://reddit.com/r/devops) — real discussions
- [r/kubernetes](https://reddit.com/r/kubernetes)
- [DevOps Discord servers](https://discord.com/invite/devops)
- [CNCF Slack](https://slack.cncf.io)

---

## Quick Reference: Weekly Study Plan (SWE-Adjusted)

Your SWE background compresses the Linux/Git phases. Focus more time on K8s, cloud, and projects.

| Week | Focus | Practical |
|------|-------|-----------|
| 1 | Linux gaps + Bash scripting | Identify and fill your Linux knowledge gaps |
| 2 | Networking fundamentals | TCP, DNS, TLS hands-on with tcpdump |
| 3 | DevOps mindset (Phase 0.5) | Study 12-Factor App, cattle-vs-pets, idempotency |
| 4 | Terraform fundamentals | Provision VPC + EC2 + S3, remote state |
| 5–6 | Terraform advanced | Modules, workspaces, CI/CD pipeline |
| 7–10 | Kubernetes core | Deploy apps, services, ingress, cert-manager |
| 11–12 | Helm + K8s storage + security | Charts, StatefulSets, RBAC, NetworkPolicies |
| 13–14 | Ansible | Configuration management |
| 15–17 | CI/CD (GitHub Actions + ArgoCD) | Full pipeline with GitOps |
| 18–21 | Cloud platform (pick one) | Deep study + certification prep |
| 22–24 | Monitoring + Observability | Prometheus, Grafana, Loki, OpenTelemetry |
| 25 | Security (DevSecOps) | SAST, container scanning, secrets management |
| 26–29 | Project 1: automated pipeline | Build end-to-end CI/CD + GitOps |
| 30–33 | Project 2: multi-tier app on K8s | Build end-to-end with security hardening |
| 34–36 | Interview prep + job search | System design, behavioral, salary negotiation |

---

## Salary & Market Outlook (2025-2026)

### Salary Ranges by Region (Junior DevOps / first role)

| Region | Junior/SRE I | Mid-Level | Senior |
|---|---|---|---|
| **US (SF/NYC/Seattle)** | $110K–$140K | $150K–$190K | $200K–$260K |
| **US (other metros)** | $90K–$120K | $130K–$165K | $170K–$220K |
| **Canada** | CAD $85K–$110K | CAD $120K–$150K | CAD $155K–$190K |
| **UK (London)** | GBP 50K–£70K | GBP 75K–£95K | GBP 100K–£130K |
| **Germany** | EUR 55K–€70K | EUR 75K–€95K | EUR 100K–€120K |
| **Netherlands** | EUR 50K–€65K | EUR 70K–€90K | EUR 95K–€115K |
| **Australia** | AUD $100K–$130K | AUD $140K–$170K | AUD $180K–$220K |
| **Singapore** | SGD $70K–$95K | SGD $100K–$140K | SGD $150K–$190K |
| **India (Bangalore)** | INR 8L–15L | INR 18L–30L | INR 35L–60L |
| **Indonesia (Jakarta)** | IDR 180M–350M | IDR 400M–700M | IDR 800M–1.5B |
| **Remote (Global)** | $80K–$120K | $130K–$170K | $180K–$240K |

Note: Remote salaries vary by company HQ location. US-based remote companies pay US-adjusted rates.

### Most In-Demand Skills (Job Posting Analysis)

Ranked by frequency in DevOps/SRE job descriptions:

1. **Kubernetes** — 85%+ of postings. CKA certification is the strongest signal.
2. **Terraform** — 75%+. OpenTofu growing but still niche.
3. **CI/CD** — GitHub Actions (60%), GitLab CI (35%), Jenkins (25% but declining).
4. **AWS** — 65%+. GCP (30%), Azure (28%). Most roles expect at least one cloud.
5. **Docker** — Assumed knowledge, rarely listed explicitly now.
6. **ArgoCD** — 35%+ and growing. GitOps is the new standard.
7. **Monitoring** — Prometheus (55%), Grafana (50%), Datadog (30%).
8. **Scripting** — Bash (ubiquitous), Python (65%), Go (40% and growing for platform engineering).
9. **Helm** — 50%+. Expected for any K8s role.
10. **Git** — Assumed. Advanced workflows (branching strategy, rebase, bisect) differentiate.

### SWE Advantage

Your programming background is a competitive edge. Many DevOps candidates come from sysadmin backgrounds and lack software engineering fundamentals. DevOps roles increasingly demand coding ability for:

- Writing Kubernetes operators (Go)
- Building internal platform tooling (Python, TypeScript)
- Infrastructure testing (Go, Python)
- CI/CD pipeline scripting (Bash, Python)
- Custom Terraform providers (Go)

Lean into this. Your SWE skills + DevOps tooling = Platform Engineer, the most lucrative DevOps-adjacent role.

---

*Remember: DevOps is a culture and practice, not a tool set. Your SWE background gives you a head start — you already think in systems, abstractions, and automation. Build real things. Break them. Fix them. That is the entire curriculum.*
