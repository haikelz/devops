# Terraform AWS Guide -- From SWE to Infrastructure Engineer

Complete, copy-paste-and-run examples for provisioning AWS with Terraform. Written for a fullstack SWE (JS/TS/Go) — every section maps infrastructure concepts to programming analogies.

---

## SWE to IaC: Mental Model Shift

Infrastructure as Code means your config files ARE the infrastructure. Just like you write functions that create data structures, you write resources that create servers, databases, and networks.

### Declarative vs Imperative

As a SWE, you are used to imperative programming: "do this, then that, then this." Terraform is **declarative**: you describe the desired end state, and Terraform figures out how to get there.

```go
// Imperative (what you are used to)
server := aws.CreateEC2("t3.micro")
server.AttachSecurityGroup("web-sg")
server.AssignIP("10.0.1.5")

// Declarative (Terraform)
resource "aws_instance" "web" {
  instance_type = "t3.micro"
  security_groups = [aws_security_group.web.name]
  private_ip = "10.0.1.5"
}
```

**Why declarative wins at scale**: You do not need to track what already exists. Terraform compares your desired state (`.tf` files) against actual state (`.tfstate`), then computes the minimal set of API calls needed. This is the same pattern as React's virtual DOM diffing.

### The State File

The most foreign concept for SWEs. `.tfstate` is a JSON file that maps your HCL resources to real AWS resource IDs. **Compare it to**: an ORM migration state table. Just like your ORM tracks which migrations have been applied so it does not re-run them, Terraform's state file tracks which resources exist so it does not re-create them.

**Critical rule**: Never edit `.tfstate` manually. Never lose it. Use remote state (S3 + DynamoDB locking) for any team environment.

### Plan Before Apply

`terraform plan` = a dry-run that shows you EXACTLY what will change. Like `git diff` before `git commit`, or `--dry-run` in kubectl. Terraform will NOT apply until you approve the plan. This is your safety net.

```
$ terraform plan
  # aws_s3_bucket.site will be created
  + resource "aws_s3_bucket" "site" {
      + bucket = "my-unique-bucket-name"
      ...
    }

Plan: 1 to add, 0 to change, 0 to destroy.
```

### The Workflow

```
terraform init    # Download provider plugins (like npm install)
terraform plan    # Dry-run diff (like git diff --staged)
terraform apply   # Execute changes (like git commit + git push)
terraform destroy # Tear everything down (like rm -rf, but for cloud)
```

---

## Setup

Install Terraform, authenticate via AWS CLI profile, env vars, or OIDC role.

---

## Project 0: Your First Terraform (15 minutes)

Before the S3+CloudFront complexity, write a single S3 bucket with zero dependencies.

```hcl
# main.tf
terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

provider "aws" { region = "us-east-1" }

resource "aws_s3_bucket" "first" {
  bucket = "my-first-tf-bucket-${random_id.suffix.hex}"
}

resource "random_id" "suffix" {
  byte_length = 4
}
```

**Walk through**:
```bash
terraform init      # Downloads AWS provider (~30MB). Creates .terraform/ directory.
terraform plan      # Shows: 1 S3 bucket to add. No changes, no destroys.
terraform apply     # Creates the bucket in AWS. Writes .tfstate file.
cat terraform.tfstate  # Look inside — see the bucket ARN, ID, region.
terraform destroy   # Deletes the bucket. .tfstate.backup remains.
```

**What `destroy` does**: Deleting the bucket. But it does NOT delete: the `.terraform/` provider cache, the `.tfstate.backup` file, or any resources you created manually in the AWS console. Only resources Terraform knows about get destroyed.

**You know it works when**: You can `apply` → `destroy` → `apply` again, and the second apply creates a fresh bucket with a different name (because `random_id` generates a new suffix each time).

---

## Project 1: S3 + CloudFront Static Site

Simple static website behind CloudFront CDN. HCL below:

    resource "aws_s3_bucket" "site" { bucket = var.bucket_name }

    resource "aws_s3_bucket_public_access_block" "site" {
      bucket = aws_s3_bucket.site.id
      block_public_acls       = true
      block_public_policy     = true
      ignore_public_acls      = true
      restrict_public_buckets = true
    }

    resource "aws_s3_object" "index" {
      bucket       = aws_s3_bucket.site.id
      key          = "index.html"
      source       = "${path.module}/site/index.html"
      content_type = "text/html"
      etag         = filemd5("${path.module}/site/index.html")
    }

    resource "aws_cloudfront_origin_access_control" "site" {
      name                              = "s3-oac-${var.bucket_name}"
      origin_access_control_origin_type = "s3"
      signing_behavior                  = "always"
      signing_protocol                  = "sigv4"
    }

    data "aws_iam_policy_document" "cloudfront_read" {
      statement {
        actions   = ["s3:GetObject"]
        resources = ["${aws_s3_bucket.site.arn}/*"]
        principals {
          type        = "Service"
          identifiers = ["cloudfront.amazonaws.com"]
        }
        condition {
          test     = "StringEquals"
          variable = "aws:SourceArn"
          values   = [aws_cloudfront_distribution.site.arn]
        }
      }
    }

    resource "aws_s3_bucket_policy" "cf" {
      bucket = aws_s3_bucket.site.id
      policy = data.aws_iam_policy_document.cloudfront_read.json
    }

    resource "aws_cloudfront_distribution" "site" {
      enabled             = true
      default_root_object = "index.html"
      aliases             = var.domain_name != "" ? [var.domain_name] : []
      origin {
        domain_name              = aws_s3_bucket.site.bucket_regional_domain_name
        origin_id                = "s3-${var.bucket_name}"
        origin_access_control_id = aws_cloudfront_origin_access_control.site.id
      }
      default_cache_behavior {
        allowed_methods  = ["GET", "HEAD"]
        cached_methods   = ["GET", "HEAD"]
        target_origin_id = "s3-${var.bucket_name}"
        viewer_protocol_policy = "redirect-to-https"
        forwarded_values {
          query_string = false
          cookies { forward = "none" }
        }
        min_ttl = 0; default_ttl = 3600; max_ttl = 86400
      }
      restrictions { geo_restriction { restriction_type = "none" } }
      viewer_certificate {
        cloudfront_default_certificate = var.domain_name != "" ? false : true
      }
    }

**Deploy**: mkdir site; terraform init; terraform plan; terraform apply

**SWE Analogy**: S3 = a filesystem in the cloud, like `fs.writeFile()` but globally distributed. CloudFront = a CDN reverse proxy, like nginx `proxy_pass` but with edge caching at 400+ locations. The bucket policy is like middleware — it checks "does this request come from CloudFront?" before serving the object.

**Why this pattern?** S3 public access is BLOCKED. Only CloudFront can read from S3 (via OAC). CloudFront handles caching, HTTPS, and custom domains. This is the "zero-trust" model applied to static files.

**Debugging**:
- `Access Denied` on CloudFront URL → Bucket policy not updated. Check `aws_s3_bucket_policy.cf` was applied. Wait 5 minutes for CloudFront propagation.
- `BucketAlreadyExists` → S3 bucket names are globally unique. Add a random suffix. All AWS accounts share the same namespace.
- `NoSuchBucket` on destroy → Someone deleted the bucket manually. Run `terraform import aws_s3_bucket.site <bucket-name>` to bring it back under Terraform management, then destroy properly.

**Cost Note**: S3 costs ~$0.023/GB stored, CloudFront ~$0.085/GB served. For learning: free tier covers 5GB S3 + 1TB CloudFront transfer for 12 months. You will likely pay $0.

**Advanced Extension**: Add a Route53 alias record to point your domain at CloudFront. Add an ACM certificate for HTTPS (free, but must be in us-east-1 region for CloudFront).

---

## Project 2: VPC from Scratch

No default VPC. Build every component manually.

    resource "aws_vpc" "main" {
      cidr_block = "10.0.0.0/16"
      enable_dns_hostnames = true
      enable_dns_support   = true
    }

    resource "aws_subnet" "public" {
      count             = length(var.azs)
      vpc_id            = aws_vpc.main.id
      cidr_block        = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index)
      availability_zone = var.azs[count.index]
      map_public_ip_on_launch = true
    }

    resource "aws_subnet" "private" {
      count             = length(var.azs)
      vpc_id            = aws_vpc.main.id
      cidr_block        = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index + 10)
      availability_zone = var.azs[count.index]
    }

    resource "aws_internet_gateway" "main" { vpc_id = aws_vpc.main.id }

    resource "aws_eip" "nat" { domain = "vpc" }

    resource "aws_nat_gateway" "main" {
      allocation_id = aws_eip.nat.id
      subnet_id     = aws_subnet.public[0].id
    }

    resource "aws_route_table" "public" {
      vpc_id = aws_vpc.main.id
      route { cidr_block = "0.0.0.0/0"; gateway_id = aws_internet_gateway.main.id }
    }

    resource "aws_route_table" "private" {
      count  = length(aws_subnet.private)
      vpc_id = aws_vpc.main.id
      route { cidr_block = "0.0.0.0/0"; nat_gateway_id = aws_nat_gateway.main.id }
    }

    resource "aws_route_table_association" "public" {
      count          = length(aws_subnet.public)
      subnet_id      = aws_subnet.public[count.index].id
      route_table_id = aws_route_table.public.id
    }

    resource "aws_route_table_association" "private" {
      count          = length(aws_subnet.private)
      subnet_id      = aws_subnet.private[count.index].id
      route_table_id = aws_route_table.private[count.index].id
    }

**Module it**: Extract to modules/vpc with variables (name, cidr, azs) and outputs (vpc_id, subnet_ids).

**Key**: cidrsubnet("10.0.0.0/16", 8, n) = /24 subnets. NAT Gateway costs ~$35/mo.

**SWE Analogy**: A VPC is like a namespace/container for your network — everything inside shares the same address space. Public subnets = routes that go through an Internet Gateway (like `0.0.0.0/0` in a route table). Private subnets = routes through a NAT Gateway (one-way: outbound only). The route table is like a router's config — it decides where packets go based on destination CIDR.

**Why this pattern?** Public subnets hold load balancers (internet-facing). Private subnets hold application servers and databases (no direct internet access). This is defense in depth — even if an attacker compromises your app server, they cannot receive inbound connections because there is no route from the internet to the private subnet.

**Debugging**:
- `InvalidSubnet.Conflict` for multiple AZs → Your `count` indexing is creating the same CIDR for all subnets. Use `cidrsubnet(cidr, 8, count.index)` to offset each subnet.
- EC2 in private subnet cannot reach internet → NAT Gateway must be in a PUBLIC subnet. Check `aws_route_table.private` routes to the NAT GW.
- `DependencyViolation` on destroy → Resources still reference the VPC (e.g., security groups). Destroy those first or add `depends_on`.

**Cost Note**: VPC itself is free. NAT Gateway costs ~$35/month + $0.045/GB data processed. For learning: provision, study, destroy within an hour. Or skip the NAT Gateway for cost-sensitive practice (your EC2 in private subnets just cannot reach the internet).



---

## Project 3: EC2 Auto Scaling + ALB

Instances in private subnets behind an ALB in public subnets. Scale on CPU.

**Launch Template**:

    data "aws_ami" "amazon_linux_2" { most_recent = true; owners = ["amazon"]
      filter { name = "name"; values = ["amzn2-ami-hvm-*-x86_64-gp2"] }
    }

    resource "aws_launch_template" "web" {
      name_prefix   = "${var.name}-web-"
      image_id      = data.aws_ami.amazon_linux_2.id
      instance_type = "t3.micro"
      vpc_security_group_ids = [aws_security_group.web.id]
      user_data = base64encode(templatefile("${path.module}/userdata.sh", {
        environment = var.environment
      }))
    }

    resource "aws_security_group" "web" {
      name = "${var.name}-web-sg"; vpc_id = var.vpc_id
      ingress { from_port = 80; to_port = 80; protocol = "tcp"
        security_groups = [aws_security_group.alb.id] }
      egress { from_port = 0; to_port = 0; protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"] }
    }

**ALB**:

    resource "aws_lb" "main" {
      name = "${var.name}-alb"; internal = false
      load_balancer_type = "application"
      security_groups = [aws_security_group.alb.id]
      subnets = var.public_subnet_ids
    }

    resource "aws_lb_target_group" "web" {
      name = "${var.name}-web-tg"; port = 80; protocol = "HTTP"; vpc_id = var.vpc_id
      health_check { path = "/"; matcher = "200-299" }
    }

    resource "aws_lb_listener" "http" {
      load_balancer_arn = aws_lb.main.arn; port = 80; protocol = "HTTP"
      default_action { type = "forward"; target_group_arn = aws_lb_target_group.web.arn }
    }

**Auto Scaling Group**:

    resource "aws_autoscaling_group" "web" {
      name = "${var.name}-web-asg"
      vpc_zone_identifier = var.private_subnet_ids
      target_group_arns = [aws_lb_target_group.web.arn]
      health_check_type = "ELB"
      min_size = 2; max_size = 6; desired_capacity = 2
      launch_template { id = aws_launch_template.web.id; version = "$Latest" }
      instance_refresh { strategy = "Rolling"
        preferences { min_healthy_percentage = 50 } }
    }

    resource "aws_autoscaling_policy" "scale_up" {
      name = "${var.name}-scale-up"
      autoscaling_group_name = aws_autoscaling_group.web.name
      adjustment_type = "ChangeInCapacity"; scaling_adjustment = 1
    }

    resource "aws_cloudwatch_metric_alarm" "cpu_high" {
      alarm_name = "${var.name}-cpu-high"
      comparison_operator = "GreaterThanThreshold"
      evaluation_periods = 2; metric_name = "CPUUtilization"
      namespace = "AWS/EC2"; period = 120; statistic = "Average"
      threshold = 70; alarm_actions = [aws_autoscaling_policy.scale_up.arn]
      dimensions = { AutoScalingGroupName = aws_autoscaling_group.web.name }
    }

**userdata.sh**: yum update; install nginx; start on boot.

---

## Project 4: RDS PostgreSQL

Stateful DB. Never in an ASG.

    resource "aws_db_subnet_group" "main" {
      name = "${var.name}-db-subnet"; subnet_ids = var.private_subnet_ids
    }

    resource "aws_db_instance" "main" {
      identifier = "${var.name}-postgres"
      engine = "postgres"; engine_version = "16.3"
      instance_class = "db.t4g.micro"
      allocated_storage = 20; max_allocated_storage = 100
      storage_encrypted = true
      db_name = var.db_name; username = var.db_username
      password = random_password.db.result; port = 5432
      db_subnet_group_name = aws_db_subnet_group.main.name
      vpc_security_group_ids = [aws_security_group.rds.id]
      publicly_accessible = false
      skip_final_snapshot = var.environment != "production"
      deletion_protection = var.environment == "production"
      backup_retention_period = var.environment == "production" ? 30 : 7
      apply_immediately = false
    }

    resource "random_password" "db" { length = 32; special = false }

    resource "aws_secretsmanager_secret" "db" { name = "${var.name}-db-password" }

    resource "aws_secretsmanager_secret_version" "db" {
      secret_id = aws_secretsmanager_secret.db.id
      secret_string = jsonencode({
        username = var.db_username; password = random_password.db.result
        host = aws_db_instance.main.address; port = 5432; dbname = var.db_name
      })
    }

**Rules**: no hard-coded passwords. Secrets in Secrets Manager. skip_final_snapshot=false and deletion_protection=true in production.

---

## Project 5: IAM Roles

Least privilege principle. Every service gets a role.

    data "aws_iam_policy_document" "ec2_assume" {
      statement { actions = ["sts:AssumeRole"]
        principals { type = "Service"; identifiers = ["ec2.amazonaws.com"] } }
    }

    resource "aws_iam_role" "ec2" {
      name = "${var.name}-ec2-role"
      assume_role_policy = data.aws_iam_policy_document.ec2_assume.json
    }

    resource "aws_iam_instance_profile" "ec2" {
      name = "${var.name}-profile"; role = aws_iam_role.ec2.name
    }

**Cross-account**: Add ExternalId condition to sts:AssumeRole trust policy.

---

## Project 6: Lambda + API Gateway + DynamoDB

Serverless stack. PAY_PER_REQUEST DynamoDB. Python Lambda behind HTTP API Gateway.

    resource "aws_dynamodb_table" "items" {
      name = "${var.name}-items"; billing_mode = "PAY_PER_REQUEST"
      hash_key = "id"; attribute { name = "id"; type = "S" }
    }

    resource "aws_lambda_function" "api" {
      filename = data.archive_file.lambda_zip.output_path
      source_code_hash = data.archive_file.lambda_zip.output_base64sha256
      function_name = "${var.name}-api"; role = aws_iam_role.lambda.arn
      handler = "index.handler"; runtime = "python3.12"
      environment { variables = { TABLE_NAME = aws_dynamodb_table.items.name } }
    }

    resource "aws_apigatewayv2_api" "main" { name = "${var.name}-api"; protocol_type = "HTTP" }

    resource "aws_apigatewayv2_stage" "main" {
      api_id = aws_apigatewayv2_api.main.id; name = "$default"; auto_deploy = true
    }

    resource "aws_apigatewayv2_integration" "lambda" {
      api_id = aws_apigatewayv2_api.main.id
      integration_type = "AWS_PROXY"
      integration_uri = aws_lambda_function.api.invoke_arn
    }

    resource "aws_apigatewayv2_route" "items" {
      api_id = aws_apigatewayv2_api.main.id
      route_key = "GET /items"
      target = "integrations/${aws_apigatewayv2_integration.lambda.id}"
    }

    resource "aws_lambda_permission" "apigw" {
      statement_id = "AllowAPIGatewayInvoke"
      action = "lambda:InvokeFunction"
      function_name = aws_lambda_function.api.function_name
      principal = "apigateway.amazonaws.com"
      source_arn = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
    }

---

## Project 7: EKS Cluster

Production-grade K8s with managed node groups.

    resource "aws_eks_cluster" "main" {
      name = var.name; role_arn = aws_iam_role.eks_cluster.arn; version = "1.30"
      vpc_config { subnet_ids = var.private_subnet_ids
        endpoint_private_access = true; endpoint_public_access = true }
    }

    resource "aws_eks_node_group" "main" {
      cluster_name = aws_eks_cluster.main.name
      node_group_name = "${var.name}-nodes"
      node_role_arn = aws_iam_role.node_group.arn
      subnet_ids = var.private_subnet_ids
      instance_types = ["t3.medium"]; capacity_type = "ON_DEMAND"
      scaling_config { desired_size = 2; max_size = 5; min_size = 1 }
    }

**Required policies**: AmazonEKSClusterPolicy, AmazonEKSWorkerNodePolicy, AmazonEKS_CNI_Policy, AmazonEC2ContainerRegistryReadOnly.

**Access**: aws eks update-kubeconfig --region us-east-1 --name my-cluster

**SWE Analogy**: EKS = managed Kubernetes control plane. AWS runs the API server, scheduler, etcd for you. You only manage worker nodes. The node group is like a Kubernetes Node auto-scaler but at the AWS level — it provisions EC2 instances that join your cluster.

**Why this pattern?** Managed K8s is the standard. Self-managed K8s (kops, kubeadm) requires you to patch, upgrade, and backup the control plane. EKS handles this. Nodes go in private subnets (no public IPs), API endpoint has both public (for kubectl) and private (for internal access) enabled.

**Debugging**:
- `Cannot connect to cluster` → Run `aws eks update-kubeconfig` first. Check that your IAM user has `eks:DescribeCluster` permission.
- Nodes show `NotReady` → Check node group IAM role has `AmazonEKSWorkerNodePolicy` and `AmazonEKS_CNI_Policy`. Missing CNI policy = pods cannot get IPs.
- `Insufficient capacity` on node group create → AWS may not have t3.medium in that AZ. Try a different instance type or AZ.

**Cost Note**: EKS control plane costs $73/month (flat fee, NOT per-node). Node group: t3.medium ~$30/month per node. A 2-node cluster = ~$133/month minimum. For learning: use GKE (free control plane) or destroy EKS after practice.

---
## Project 9: Multi-Environment Terraform (Production Patterns)

A single Terraform workspace is not enough for dev/staging/prod. You need environment isolation.

### Approach 1: Directory-per-Environment (Recommended)

```
terraform/
├── environments/
│   ├── dev/
│   │   ├── main.tf      # Calls modules with dev-specific values
│   │   └── terraform.tfvars
│   ├── staging/
│   │   ├── main.tf
│   │   └── terraform.tfvars
│   └── prod/
│       ├── main.tf
│       └── terraform.tfvars
└── modules/
    ├── vpc/
    ├── eks/
    └── rds/
```

Each environment has its own state file (different S3 backend key):
```hcl
# environments/prod/backend.tf
terraform {
  backend "s3" {
    bucket = "myco-terraform-state"
    key    = "prod/terraform.tfstate"  # Different key per env!
    region = "us-east-1"
  }
}
```

### Approach 2: Terraform Workspaces

```bash
terraform workspace new dev
terraform workspace new staging
terraform workspace new prod
terraform workspace select prod && terraform apply
```

Workspaces create separate state files under the same backend. Simpler for small teams, but ALL environments share the same backend config and credentials — one mistake can affect prod.

### tfvars Pattern

```hcl
# terraform.tfvars (dev)
environment = "dev"
instance_count = 1
instance_type = "t3.micro"

# terraform.tfvars (prod)
environment = "prod"
instance_count = 3
instance_type = "t3.medium"
```

### The Blast Radius Concept

**NEVER share state between environments.** If your dev Terraform accidentally deletes a resource, and that resource's state is shared with prod, your prod goes down too.

Rules:
- Separate state files per environment (different S3 keys or different workspaces)
- Separate AWS accounts for prod (or at minimum separate IAM roles)
- `terraform destroy` only in dev/staging. Prod destroy should require manual approval in CI
- Use `prevent_destroy = true` lifecycle rule on production data stores (RDS, S3, DynamoDB)

### Module Versioning

Pin module versions with Git tags:

```hcl
module "vpc" {
  source = "github.com/myorg/terraform-modules//vpc?ref=v1.2.0"
  # Never use ?ref=main in production
}
```

---

## Remote State: S3 Backend + DynamoDB Locking

Shared state with locking for team environments.

    terraform { backend "s3" { bucket = "myco-state"; key = "prod/vpc/terraform.tfstate"
      region = "us-east-1"; encrypt = true; dynamodb_table = "terraform-locks" } }

Bootstrap once per account: S3 bucket with versioning, SSE-S3, public access blocked. DynamoDB table with LockID hash key, PAY_PER_REQUEST.

---

## Patterns & Best Practices

**locals**: Define common_tags and prefix once, merge everywhere via merge().

**data sources**: aws_availability_zones, aws_route53_zone, aws_ami for dynamic lookups.

**lifecycle**: prevent_destroy (safety), ignore_changes (password), create_before_destroy (zero-downtime).

**count vs for_each**: Use for_each by default -- preserves resources on add/remove. Count re-creates everything on list changes.

**dynamic blocks**: Use for repeating blocks like security group ingress rules.

**conditional resources**: count = var.enabled ? 1 : 0 for optional resources.

**depends_on**: Only when Terraform can't infer the dependency (e.g., S3 policy depends on CloudFront).

### SWE Anti-Patterns (What You Will Do Wrong)

1. **Hardcoding ARNs/IDs** — You write `subnet-abc123` because copying from the console is fast. Next week, someone recreates the subnet with a new ID, and your config breaks. Use `data` sources or resource references (`aws_subnet.private[0].id`).

2. **Manual console changes** — You add a security group rule in the AWS console to "quickly test something." Then `terraform plan` wants to remove it. Terraform is the source of truth. The console is read-only. If you need the rule, put it in the `.tf` file.

3. **Not using data sources** — You look up the AMI ID in the console (`ami-0c55b159cbfafe1f0`), paste it into your Terraform config. This AMI is region-specific and will be deprecated. Use:

```hcl
data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]
  filter { name = "name"; values = ["al2023-ami-*-x86_64"] }
}
```

4. **Terraform + manual changes in the same account** — You provision a VPC with Terraform, then create an EC2 in the console. Terraform does not know about that EC2, and you cannot reproduce it. Either Terraform manages everything, or nothing. Pick one.

### DRY in Terraform

- **locals**: Define once, reference everywhere. Common tags, naming prefixes, environment name.
- **Modules**: Reusable infrastructure components. Write once, instantiate per environment via `source` and `variables`.
- **Terragrunt**: A thin wrapper around Terraform that keeps your configs DRY at the environment level. Useful when managing 10+ environments.

### Multi-Region Tips

```hcl
provider "aws" { alias = "us-east-1"; region = "us-east-1" }
provider "aws" { alias = "eu-west-1"; region = "eu-west-1" }

resource "aws_s3_bucket" "primary"   { provider = aws.us-east-1; ... }
resource "aws_s3_bucket" "replica"   { provider = aws.eu-west-1; ... }
```

Use `for_each` over regions:
```hcl
variable "regions" { type = set(string); default = ["us-east-1", "eu-west-1"] }
resource "aws_s3_bucket" "multi" { for_each = var.regions; bucket = "my-app-${each.key}"; ... }
```


---

## Project 8: Terraform CI/CD Deep-Dive

Automate plan/apply to prevent manual mistakes and enforce review.

### GitLab CI (Full Pipeline)

```yaml
# .gitlab-ci.yml
stages:
  - validate
  - plan
  - apply

.terraform_base: &terraform_base
  image: hashicorp/terraform:1.9
  before_script:
    - cd ${CI_ENVIRONMENT_NAME}
    - terraform init

fmt_validate:
  stage: validate
  <<: *terraform_base
  script:
    - terraform fmt -check -recursive
    - terraform validate
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"

plan:
  stage: plan
  <<: *terraform_base
  script:
    - terraform plan -out=plan.tfplan
  artifacts:
    paths:
      - ${CI_ENVIRONMENT_NAME}/plan.tfplan
    expire_in: 7 days
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"

apply:
  stage: apply
  <<: *terraform_base
  script:
    - terraform apply plan.tfplan
  when: manual  # Requires a human to click "Run"
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
```

**Stage-by-stage**: `validate` catches syntax errors, `plan` generates the diff (saved as an artifact for review), `apply` requires manual approval and only runs on main branch merges.

### GitHub Actions with OIDC

OIDC (OpenID Connect) lets GitHub Actions assume an AWS IAM role WITHOUT storing AWS credentials as secrets. This is the modern standard.

```yaml
# .github/workflows/terraform.yml
name: Terraform
on:
  pull_request:
    paths: ['terraform/**']
  push:
    branches: [main]
    paths: ['terraform/**']

permissions:
  id-token: write   # Required for OIDC
  contents: read

jobs:
  terraform:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        environment: [dev, staging]
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::ACCOUNT:role/github-actions-terraform
          aws-region: us-east-1

      - run: terraform init
        working-directory: terraform/environments/${{ matrix.environment }}

      - run: terraform plan -out=plan.tfplan
        working-directory: terraform/environments/${{ matrix.environment }}

      # On push to main, auto-apply plan
      - if: github.event_name == 'push' && matrix.environment != 'prod'
        run: terraform apply plan.tfplan
        working-directory: terraform/environments/${{ matrix.environment }}
```

### Atlantis (PR-Based Plan/Apply)

Atlantis runs `terraform plan` automatically on PR creation and posts the plan as a PR comment. Apply happens by commenting `atlantis apply` on the PR.

```hcl
# atlantis.yaml
version: 3
projects:
  - name: dev
    dir: terraform/environments/dev
    workflow: default
  - name: staging
    dir: terraform/environments/staging
    workflow: default
  - name: prod
    dir: terraform/environments/prod
    workflow: default
    apply_requirements: [approved, mergeable]  # Requires PR approval for prod
```

### How to Review a Terraform Plan in a PR

When someone opens a PR with Terraform changes, look for:

1. **Destroy/Create** — `+` means create, `-` means destroy, `~` means modify in-place. Any `-` on a database, DNS record, or load balancer = red flag. Ask: is this intentional?
2. **Security group changes** — Adding `0.0.0.0/0` ingress = anyone can access. Is this on the right port? Should it be restricted to a specific CIDR?
3. **IAM changes** — Any new `aws_iam_policy` or `aws_iam_role` — does it follow least privilege? Does `"Resource": "*"` need to be scoped?
4. **Cost impact** — New NAT Gateway? New RDS instance? These have fixed costs ($35-100/month). Is this budgeted?
5. **State drift** — If the plan shows changes you did not expect (resources Terraform wants to modify that the PR did not touch), someone made manual console changes. Ask them to `terraform import` or clean up.



---

## Learning Path

0. **Project 0** — understand init/plan/apply/destroy, state file, provider basics
1. **S3 + CloudFront** — understand providers, resources, data sources, outputs
2. **VPC from scratch** — understand networking, modules, count/for_each
3. **EC2 + ALB + ASG** — understand compute, load balancing, scaling
4. **RDS + Secrets Manager** — understand stateful resources, security
5. **IAM** — understand least privilege, roles, policies
6. **Lambda + API Gateway** — understand serverless patterns
7. **EKS** — understand managed K8s on AWS
8. **Multi-Environment** — understand workspaces, tfvars, blast radius, module versioning
9. **CI/CD** — automate plan/apply, PR-based review, OIDC auth

---

## Resources

- Terraform AWS Provider Docs: registry.terraform.io/providers/hashicorp/aws
- AWS Provider Guides: learn.hashicorp.com/tutorials/terraform
- AWS Well-Architected Framework
