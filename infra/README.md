# SplitLedger — AWS CDK Infrastructure

**Region:** `ap-southeast-1` (Singapore)  
**Stacks:** Network → Data → App → Pipeline

## Architecture

```text
Browser / Vercel
       │ HTTPS
       ▼
Lambda Function URL (splitleger-api, provided.al2023)
       │
       ├── Aiven PostgreSQL over TLS
       ├── ElastiCache Redis in isolated subnets
       ├── S3 receipts (pre-signed URLs)
       ├── SES transactional email
       └── encrypted SSM parameters at cold start

GitHub Actions (OIDC) → Lambda UpdateFunctionCode
```

The Lambda function runs in private isolated subnets. One NAT gateway provides outbound access to Aiven and AWS APIs; the S3 gateway endpoint remains free. Redis accepts traffic only from the Lambda security group.

## Stacks

| Stack                 | Resources                                                                                                     |
| --------------------- | ------------------------------------------------------------------------------------------------------------- |
| `splitleger-network`  | VPC, public/isolated subnets, one NAT gateway, Lambda and Redis security groups, S3 gateway endpoint          |
| `splitleger-data`     | ElastiCache Redis, locked-down S3 receipts bucket, eight `SecureString` parameters, two non-secret parameters |
| `splitleger-app`      | Native Go Lambda Function URL, VPC placement, SSM/KMS/S3/SES permissions, CloudWatch log group                |
| `splitleger-pipeline` | GitHub OIDC provider and main-branch-only deployment role                                                     |

## Prerequisites

- AWS CLI configured with an account that can deploy CDK stacks.
- Node.js 22+, npm, Docker, and AWS CDK CLI dependencies installed in `infra/`.
- Aiven PostgreSQL database and verified SES sender identity.
- A Route 53 hosted zone for the configured frontend domain if DNS is managed separately.

```bash
cd infra
npm ci
npm run build
```

The Lambda code is bundled by CDK from `api/` with a Go 1.25 Docker image. CDK deployment requires Docker; normal API/web development does not.

## First deployment

Review locally before any AWS operation:

```bash
cd infra
npx cdk synth
npx cdk diff --all
```

Bootstrap once per account/region, then deploy all stacks in dependency order:

```bash
npx cdk bootstrap aws://ACCOUNT_ID/ap-southeast-1
npx cdk deploy --all --require-approval broadening
```

The app stack outputs `ApiFunctionUrl`. Set the Vercel variables:

```text
NEXT_PUBLIC_API_URL=<ApiFunctionUrl>
NEXT_PUBLIC_GRAPHQL_URL=<ApiFunctionUrl>/graphql
```

Do not put database credentials, JWT keys, OAuth secrets, or SES values in Lambda environment variables or GitHub secrets. The API loads them from encrypted SSM parameters during a cold start.

## Populate SecureString parameters

The data stack creates placeholders so the first deployment is deterministic. Replace every required placeholder before invoking the function:

```bash
aws ssm put-parameter --name /splitleger/db_url --type SecureString --overwrite \
  --value "postgresql://user:pass@host.aiven.io:5432/defaultdb?sslmode=require"
aws ssm put-parameter --name /splitleger/redis_url --type SecureString --overwrite \
  --value "redis://REDIS_ENDPOINT:6379"

openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem
aws ssm put-parameter --name /splitleger/jwt_private_key --type SecureString --overwrite --value "$(cat private.pem)"
aws ssm put-parameter --name /splitleger/jwt_public_key --type SecureString --overwrite --value "$(cat public.pem)"

aws ssm put-parameter --name /splitleger/aiven_ca_cert --type SecureString --overwrite --value "$(cat ca.pem)"
aws ssm put-parameter --name /splitleger/google_client_id --type SecureString --overwrite --value "GOOGLE_CLIENT_ID"
aws ssm put-parameter --name /splitleger/google_client_secret --type SecureString --overwrite --value "GOOGLE_CLIENT_SECRET"
aws ssm put-parameter --name /splitleger/email_from --type SecureString --overwrite --value "SplitLedger <no-reply@example.com>"
```

`AIVEN_CA_CERT` is optional at runtime for the current pgx configuration, but keeping it encrypted makes the parameter contract explicit. The API fails fast when a required parameter is missing or still contains `REPLACE_ME`.

## GitHub Actions

After `SplitlegerPipeline` deploys, copy its `GithubActionsRoleArn` output into the GitHub repository secret:

```text
AWS_DEPLOY_ROLE_ARN
```

The role trusts only:

```text
repo:Ke-vin-S/ledger:ref:refs/heads/main
```

The deploy workflow builds `api/cmd/api` as a native `bootstrap` binary, updates `splitleger-api`, and waits for the update. It does not use ECR, ECS, static AWS keys, or the removed out-of-band setup script.

## Operations

```bash
# Synthesize locally (no deploy)
npx cdk synth

# Preview changes
npx cdk diff --all

# Deploy one stack
npx cdk deploy SplitlegerApp

# Tail API logs
aws logs tail /aws/lambda/splitleger-api --follow --region ap-southeast-1

# Check health through the Function URL returned by the AppStack
API_URL="<ApiFunctionUrl>"
curl -fsS "$API_URL/health"
curl -fsS "$API_URL/ready"
```

The `/ready` endpoint verifies both PostgreSQL and Redis.

## Security notes

- S3 blocks all public access and requires TLS.
- SSM `SecureString` values are decrypted only by the Lambda execution role through KMS.
- GitHub Actions uses OIDC and a main-branch-only trust policy; no long-lived AWS keys are required.
- CloudWatch logs retain seven days; audit rows remain append-only in PostgreSQL.
- `cdk destroy` is intentionally blocked by termination protection until explicitly disabled in AWS.
