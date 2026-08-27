# GKE infrastructure

This Terraform root provisions the GCP infrastructure for the manifests in `../k8s`:

- a dedicated VPC and subnet with alias IP ranges for Pods and Services;
- a regional private-node GKE cluster with Workload Identity and network-policy enforcement;
- Cloud NAT so private nodes can reach required public services without public node IPs;
- a managed, shielded node pool.

Application resources remain in `k8s/` and are reconciled through Argo CD. Keeping cloud infrastructure and application delivery separate avoids Terraform state owning application Secrets or competing with GitOps.

## Prerequisites

- Terraform 1.6 or newer
- Google Cloud SDK
- an authenticated GCP identity with permission to create GKE and VPC resources

Authenticate with Application Default Credentials:

```bash
gcloud auth application-default login
```

## Create the cluster

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with the GCP project and authorized administrator/CI CIDRs.
terraform init
terraform plan
terraform apply
terraform output -raw get_credentials_command
```

Run the printed command, then bootstrap the platform components and applications from `../k8s`.

`terraform.tfstate` is currently local state. Move it to a versioned, access-controlled GCS backend before collaborators or CI share this infrastructure.
