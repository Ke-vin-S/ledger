# SplitLedger — AWS CDK Infrastructure

AWS CDK TypeScript for `ap-southeast-1` (Singapore). Four stacks: Network → Data → App → Pipeline.

## Commands

```bash
npm ci
npm run build                 # TypeScript
npm test                      # CDK assertions (bundles the Go Lambda locally)
npx cdk synth                 # local CloudFormation synthesis
npx cdk diff --all            # review before any deploy
npx cdk deploy --all --require-approval broadening
npx cdk deploy SplitlegerApp  # one stack
```

`npm test` and `cdk synth` use Docker to bundle the Go API. They do not invoke AWS Lambda or deploy resources. `cdk diff`/`cdk deploy` are the commands that contact AWS; review their output before running them.

## Structure

```text
bin/splitleger-infra.ts   # stack wiring
lib/
  config.ts               # region, domain, sizing, and tags
  network-stack.ts        # VPC, NAT, Lambda/Redis security groups
  data-stack.ts            # Redis, S3, SecureString SSM parameters
  app-stack.ts             # Go Lambda Function URL and least-privilege IAM
  pipeline-stack.ts        # GitHub OIDC main-branch deploy role
```

## Stack dependency order

`SplitlegerNetwork` → `SplitlegerData` → `SplitlegerApp` → `SplitlegerPipeline`

## Architecture

- Lambda runs in isolated subnets with one NAT gateway for outbound Aiven/API access.
- Redis is isolated and accepts traffic only from the Lambda security group.
- S3 uses a gateway VPC endpoint and blocks all public access.
- The API reads encrypted SSM `SecureString` values at cold start; no secret values are placed in Lambda environment variables.
- Function URL CORS allows only the configured frontend origin and credentialed requests.
- GitHub Actions uses OIDC and trusts only `repo:Ke-vin-S/ledger:ref:refs/heads/main`.

## Configuration

`lib/config.ts` is the source of truth. Set the real frontend domain before deployment. The GitHub repository is wired in `bin/splitleger-infra.ts` and the pipeline role is main-branch-only.

## SSM parameters

The data stack creates ten parameters. Eight are `SecureString`; `s3_bucket` and `app_env` are ordinary strings. Required placeholders must be replaced with `aws ssm put-parameter --type SecureString --overwrite` before invoking the function. Required names:

- `/splitleger/db_url`
- `/splitleger/redis_url`
- `/splitleger/jwt_private_key`
- `/splitleger/jwt_public_key`
- `/splitleger/google_client_id`
- `/splitleger/google_client_secret`
- `/splitleger/s3_bucket`

Optional encrypted parameters are `/splitleger/aiven_ca_cert` and `/splitleger/email_from`.

## Gotchas

- `cdk bootstrap aws://ACCOUNT_ID/ap-southeast-1` is required once per account/region.
- All stacks use termination protection; disable it manually before any destroy.
- The app stack bundles `api/` with Go 1.25 Docker; Docker is required at deploy/synthesis time.
- `ApiFunctionUrl` is the public API endpoint. Set Vercel's `NEXT_PUBLIC_API_URL` to that output.
- `api/scripts/setup-lambda.sh` is intentionally absent; the CDK stack and deploy workflow own Lambda creation and code updates.

## Never Do

- Never put secret values in `lib/config.ts` or TypeScript source.
- Never modify generated CloudFormation in `cdk.out/`.
- Never deploy `SplitlegerApp` before `SplitlegerData`; it imports the SSM parameter ARNs.
- Never add static AWS credentials to GitHub; use the OIDC role.

## Reference

- `README.md` — complete local and AWS deployment runbook
- `.github/workflows/deploy-api.yml` — test, migration, and Lambda code deployment
