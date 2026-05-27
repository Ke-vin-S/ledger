# SplitLedger — AWS CDK Infrastructure

**Region:** ap-southeast-1 (Singapore)  
**Stack count:** 4 (Network → Data → App → Pipeline)

---

## Architecture

```
Internet
    │
    ▼
Route 53  (api.splitleger.app)
    │
    ▼
ACM Certificate (TLS 1.2+)
    │
    ▼
ALB (public subnets, 2 AZs)
  ├── :80  → redirect to HTTPS
  └── :443 → ECS target group
         │
         ▼
ECS Fargate (public subnets, no public IP)
  Go API container
  0.25 vCPU / 512 MB
         │
         ├──→ Aiven PostgreSQL (external, TLS)
         ├──→ ElastiCache Redis (isolated subnets)
         └──→ S3 (receipt images, via pre-signed URLs)

GitHub Actions (OIDC) → ECR push → ECS update-service
```

---

## Stacks

| Stack | Resources |
|---|---|
| `splitleger-network` | VPC, 2 public + 2 isolated subnets, 3 security groups, S3 gateway endpoint |
| `splitleger-data` | ElastiCache Redis, S3 bucket, SSM parameters, CloudWatch log group |
| `splitleger-app` | ECR, ECS cluster + Fargate service, ALB, ACM certificate, Route 53 A record, IAM task roles |
| `splitleger-pipeline` | GitHub OIDC provider, GitHub Actions IAM role |

---

## Prerequisites

Before running `cdk deploy`:

1. **AWS CLI configured**
   ```bash
   aws configure
   # or use AWS SSO / environment variables
   ```

2. **CDK bootstrapped** (once per account/region)
   ```bash
   cdk bootstrap aws://ACCOUNT_ID/ap-southeast-1
   ```

3. **Route 53 hosted zone exists** for `splitleger.app`  
   The AppStack looks up the hosted zone by domain name.  
   Create it manually first:
   ```bash
   aws route53 create-hosted-zone --name splitleger.app --caller-reference $(date +%s)
   ```
   Then update your domain registrar's NS records to point to the Route 53 nameservers.

4. **Install dependencies**
   ```bash
   npm install
   npm run build
   ```

5. **Edit `bin/splitleger-infra.ts`**  
   Replace `YOUR_GITHUB_USERNAME/splitleger-api` with your actual GitHub repo.

6. **Edit `lib/config.ts`**  
   Replace `splitleger.app` with your actual domain name.

---

## Deployment

### First deploy (all stacks)
```bash
# Preview what will be created
cdk diff --all

# Deploy in order (CDK resolves dependencies automatically)
cdk deploy --all --require-approval broadening
```

### After first deploy — update SSM parameters
The data stack creates SSM parameters with placeholder values.  
Update them with real secrets before starting the ECS service:

```bash
# Aiven Postgres connection string
aws ssm put-parameter \
  --name /splitleger/db_url \
  --value "postgresql://user:pass@host.aiven.io:5432/defaultdb?sslmode=require" \
  --type SecureString \
  --overwrite

# Redis URL (use the endpoint from DataStack output)
aws ssm put-parameter \
  --name /splitleger/redis_url \
  --value "redis://ELASTICACHE_ENDPOINT:6379" \
  --type SecureString \
  --overwrite

# Generate RS256 keypair for JWT
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem

aws ssm put-parameter \
  --name /splitleger/jwt_private_key \
  --value "$(cat private.pem)" \
  --type SecureString \
  --overwrite

aws ssm put-parameter \
  --name /splitleger/jwt_public_key \
  --value "$(cat public.pem)" \
  --type SecureString \
  --overwrite

# Aiven CA certificate
aws ssm put-parameter \
  --name /splitleger/aiven_ca_cert \
  --value "$(cat ca.pem)" \
  --type SecureString \
  --overwrite

# Google OAuth credentials
aws ssm put-parameter \
  --name /splitleger/google_client_id \
  --value "YOUR_GOOGLE_CLIENT_ID" \
  --type SecureString \
  --overwrite

aws ssm put-parameter \
  --name /splitleger/google_client_secret \
  --value "YOUR_GOOGLE_CLIENT_SECRET" \
  --type SecureString \
  --overwrite
```

### Force new ECS deployment (after pushing a new image)
```bash
aws ecs update-service \
  --cluster splitleger \
  --service splitleger-api \
  --force-new-deployment \
  --region ap-southeast-1
```

### Update a single stack
```bash
cdk deploy SplitlegerApp
```

### Diff before deploying
```bash
cdk diff SplitlegerApp
```

---

## GitHub Actions Setup

After deploying `SplitlegerPipeline`, the stack outputs the GitHub Actions role ARN.  
Add these secrets to your GitHub repository:

| Secret | Value |
|---|---|
| `AWS_ROLE_ARN` | Output: `GithubActionsRoleArn` from PipelineStack |
| `ECR_REGISTRY` | Output: `EcrRepositoryUri` from PipelineStack |
| `AWS_REGION` | `ap-southeast-1` |

The GitHub Actions workflow in `splitleger-api/.github/workflows/deploy.yml`  
uses `role-to-assume: ${{ secrets.AWS_ROLE_ARN }}` — no static AWS keys needed.

---

## Cost Estimate (monthly)

| Resource | Cost |
|---|---|
| ALB | ~$16–18 |
| ECS Fargate (1 task, 0.25 vCPU / 512 MB) | ~$4–5 |
| ElastiCache cache.t3.micro | $0 (free tier 12 months), then ~$12 |
| ECR | $0 (free tier 500 MB) |
| S3 receipts | $0 (free tier 5 GB) |
| CloudWatch Logs | $0 (7-day retention, free tier) |
| SSM Parameter Store | $0 (standard params) |
| **Total** | **~$20–23/month** |

### To reduce the ALB cost
Replace the ALB with a `t3.micro` EC2 running Caddy as a reverse proxy.  
- EC2 t3.micro: $0 for 12 months (free tier), then ~$8.50/month
- Caddy handles TLS termination automatically via Let's Encrypt
- Point Caddy's reverse proxy at the ECS task via service discovery or static IP

---

## Useful Commands

```bash
# List all stacks
cdk ls

# Synthesise CloudFormation templates (dry run, no deploy)
cdk synth

# Show what would change
cdk diff --all

# Deploy specific stack
cdk deploy SplitlegerNetwork

# ECS Exec — SSH into a running container
aws ecs execute-command \
  --cluster splitleger \
  --task TASK_ID \
  --container api \
  --interactive \
  --command "/bin/sh"

# View recent logs
aws logs tail /ecs/splitleger-api --follow

# Check ECS service status
aws ecs describe-services \
  --cluster splitleger \
  --services splitleger-api \
  --region ap-southeast-1
```

---

## Security Notes

- All secrets injected via SSM at container startup. No secrets in Docker images or env files.
- ECS tasks have no public IP. Only reachable via ALB security group.
- Task role uses least-privilege — only S3 (specific bucket), SSM (specific params), CloudWatch.
- GitHub Actions uses OIDC — no long-lived AWS credentials in GitHub secrets.
- ACM certificate uses DNS validation — auto-renews without manual intervention.
- Redis is in isolated subnets with a security group that only allows ECS inbound.
- S3 bucket blocks all public access. Pre-signed URLs are time-limited (15 min).
- ALB enforces TLS 1.2+ (TLS13_RES policy).
