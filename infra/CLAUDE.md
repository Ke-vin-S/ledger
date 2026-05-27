# SplitLedger — AWS CDK Infrastructure

AWS CDK TypeScript. Deploys to ap-southeast-1 (Singapore). Four stacks: Network → Data → App → Pipeline.

## Commands

```bash
# Install
npm install

# Build TypeScript
npm run build

# Preview changes (always run before deploy)
cdk diff --all

# Deploy all stacks (in dependency order)
cdk deploy --all --require-approval broadening

# Deploy single stack
cdk deploy SplitlegerNetwork
cdk deploy SplitlegerData
cdk deploy SplitlegerApp
cdk deploy SplitlegerPipeline

# Synthesise CloudFormation (dry run, no deploy)
cdk synth

# List stacks
cdk ls
```

## Structure

```
bin/splitleger-infra.ts   # entrypoint — instantiates all stacks
lib/
  config.ts               # SINGLE SOURCE OF TRUTH for all config values
  network-stack.ts        # VPC, subnets, security groups
  data-stack.ts           # ElastiCache Redis, S3, SSM params, CloudWatch log group
  app-stack.ts            # ECR, ECS Fargate, ALB, ACM, Route 53, IAM roles
  pipeline-stack.ts       # GitHub OIDC provider, GitHub Actions deploy role
```

## Stack dependency order

`SplitlegerNetwork` → `SplitlegerData` → `SplitlegerApp` → `SplitlegerPipeline`

CDK resolves cross-stack references automatically. Deploy with `--all` and let CDK sequence them.

## Architecture

- No NAT Gateway (saves ~$32/month). ECS tasks run in public subnets with no public IP — reachable only via ALB security group.
- S3 Gateway VPC endpoint is free and required for ECS to reach ECR (ECR layers are stored in S3).
- ElastiCache Redis in isolated subnets — no internet access, only reachable from ECS task security group.
- All secrets live in SSM Parameter Store. ECS injects them as env vars at task startup via the `secrets` field in task definition. Never hardcode secrets or put them in environment variables directly.
- GitHub Actions uses OIDC — no static AWS keys. The `SplitlegerPipeline` stack outputs the role ARN to use in the workflow.

## Config

All values in `lib/config.ts`. Change there — propagates everywhere. Key values to update before first deploy:

- `domainName` — your actual domain (currently `splitleger.app`)
- GitHub repo in `bin/splitleger-infra.ts` — currently set to `Ke-vin-S/ledger`

## SSM Parameters

Data stack creates parameters with `REPLACE_ME` placeholder values. After first deploy, update with real secrets:

```bash
aws ssm put-parameter --name /splitleger/db_url --value "..." --type SecureString --overwrite
aws ssm put-parameter --name /splitleger/redis_url --value "..." --type SecureString --overwrite
aws ssm put-parameter --name /splitleger/jwt_private_key --value "$(cat private.pem)" --type SecureString --overwrite
aws ssm put-parameter --name /splitleger/jwt_public_key --value "$(cat public.pem)" --type SecureString --overwrite
aws ssm put-parameter --name /splitleger/google_client_id --value "..." --type SecureString --overwrite
aws ssm put-parameter --name /splitleger/google_client_secret --value "..." --type SecureString --overwrite
aws ssm put-parameter --name /splitleger/aiven_ca_cert --value "$(cat ca.pem)" --type SecureString --overwrite
```

## Gotchas

- Route 53 hosted zone must exist before deploying `SplitlegerApp`. The stack does a zone lookup — it fails if the zone doesn't exist yet.
- All three stacks have `terminationProtection: true`. You must disable it manually in the console before `cdk destroy`.
- `cdk bootstrap` must be run once per account/region before any deploy: `cdk bootstrap aws://ACCOUNT_ID/ap-southeast-1`.
- ECS task definition generates a new revision on every `cdk deploy`. This is expected — Fargate does a rolling update only if the image tag changed.
- `cdk diff` on `SplitlegerApp` will show IAM changes on every run due to dynamic ARN resolution. Review carefully — don't approve unexpected permission additions.

## Never Do

- Never put secret values in `lib/config.ts` or any TypeScript source file.
- Never modify generated CloudFormation in `cdk.out/` — it is regenerated on every `cdk synth`.
- Never deploy `SplitlegerApp` before `SplitlegerData` — the app stack imports SSM parameter ARNs from the data stack.

## Reference

- @docs/tech-stack.docx — infrastructure architecture diagram, cost breakdown, full ECS task definition, Dockerfile
- README.md — full deployment runbook with all commands
