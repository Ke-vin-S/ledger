#!/bin/bash
# api/scripts/setup-lambda.sh
#
# One-time setup: creates the Lambda function, attaches the Lambda Web Adapter
# layer, enables a Function URL, and creates the GitHub Actions OIDC deploy role.
#
# Run this once before the first CI deploy. Safe to re-run — existing resources
# are left unchanged (create-if-not-exists pattern).
#
# Prerequisites:
#   aws CLI configured with admin credentials
#   jq installed (brew install jq / apt install jq)
#
# Usage:
#   GITHUB_ORG=Ke-vin-S GITHUB_REPO=ledger ./api/scripts/setup-lambda.sh
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────

FUNCTION_NAME="splitleger-api"
REGION="ap-south-1"
RUNTIME="provided.al2023"
TIMEOUT=30      # seconds — raise if your p99 ever approaches this
MEMORY=512      # MB

# Lambda Web Adapter public layer (x86_64, ap-south-1).
# Check latest at: https://github.com/awslabs/aws-lambda-go-api-proxy
# or: aws lambda list-layer-versions --layer-name LambdaAdapterLayerX86 --region ap-south-1
ADAPTER_LAYER="arn:aws:lambda:ap-south-1:753240598075:layer:LambdaAdapterLayerX86:24"

GITHUB_ORG="${GITHUB_ORG:-Ke-vin-S}"
GITHUB_REPO="${GITHUB_REPO:-ledger}"

# ── Resolve account ───────────────────────────────────────────────────────────

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
echo "Account: $ACCOUNT_ID  Region: $REGION"

# ── Lambda execution role ─────────────────────────────────────────────────────

EXEC_ROLE_NAME="${FUNCTION_NAME}-exec-role"
EXEC_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${EXEC_ROLE_NAME}"

if ! aws iam get-role --role-name "$EXEC_ROLE_NAME" &>/dev/null; then
  echo "Creating Lambda execution role..."
  aws iam create-role \
    --role-name "$EXEC_ROLE_NAME" \
    --assume-role-policy-document '{
      "Version": "2012-10-17",
      "Statement": [{
        "Effect": "Allow",
        "Principal": { "Service": "lambda.amazonaws.com" },
        "Action": "sts:AssumeRole"
      }]
    }' >/dev/null

  aws iam attach-role-policy \
    --role-name "$EXEC_ROLE_NAME" \
    --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

  echo "Waiting for role to propagate..."
  sleep 10
else
  echo "Execution role already exists — skipping."
fi

# ── Lambda function ───────────────────────────────────────────────────────────

# Build a minimal placeholder zip so the function can be created.
# The CI pipeline will overwrite this on the first real deploy.
TMPDIR=$(mktemp -d)
echo '#!/bin/sh' > "$TMPDIR/bootstrap"
chmod +x "$TMPDIR/bootstrap"
(cd "$TMPDIR" && zip function.zip bootstrap) >/dev/null

if ! aws lambda get-function --function-name "$FUNCTION_NAME" --region "$REGION" &>/dev/null; then
  echo "Creating Lambda function..."
  aws lambda create-function \
    --function-name "$FUNCTION_NAME" \
    --runtime "$RUNTIME" \
    --role "$EXEC_ROLE_ARN" \
    --handler bootstrap \
    --zip-file "fileb://$TMPDIR/function.zip" \
    --timeout "$TIMEOUT" \
    --memory-size "$MEMORY" \
    --architectures x86_64 \
    --region "$REGION" >/dev/null

  aws lambda wait function-active \
    --function-name "$FUNCTION_NAME" \
    --region "$REGION"

  echo "Function created."
else
  echo "Lambda function already exists — skipping create."
fi

rm -rf "$TMPDIR"

# ── Lambda Web Adapter layer + env vars ───────────────────────────────────────
# The Lambda Web Adapter extension starts alongside your binary, converts
# API Gateway / Function URL events to plain HTTP requests on PORT 8080,
# and forwards responses back. No Go code changes needed.
#
# Set the real secret env vars after this script via the AWS console or:
#   aws lambda update-function-configuration \
#     --function-name splitleger-api \
#     --environment "Variables={...,DATABASE_URL=<value>,...}" \
#     --region ap-south-1

echo "Attaching Lambda Web Adapter layer and setting env vars..."
aws lambda update-function-configuration \
  --function-name "$FUNCTION_NAME" \
  --layers "$ADAPTER_LAYER" \
  --timeout "$TIMEOUT" \
  --environment "Variables={
    PORT=8080,
    ENV=production,
    FRONTEND_URL=https://ledger.kevinsanjula.me,
    DATABASE_URL=REPLACE_ME,
    REDIS_URL=REPLACE_ME,
    JWT_PRIVATE_KEY=REPLACE_ME,
    JWT_PUBLIC_KEY=REPLACE_ME,
  }" \
  --region "$REGION" >/dev/null

aws lambda wait function-updated \
  --function-name "$FUNCTION_NAME" \
  --region "$REGION"

echo "Layer and env vars set. Update the REPLACE_ME values in the Lambda console or via:"
echo "  aws lambda update-function-configuration --function-name $FUNCTION_NAME --environment ..."

# ── Function URL ──────────────────────────────────────────────────────────────

if ! aws lambda get-function-url-config \
     --function-name "$FUNCTION_NAME" \
     --region "$REGION" &>/dev/null; then
  echo "Creating Function URL..."
  FUNCTION_URL=$(aws lambda create-function-url-config \
    --function-name "$FUNCTION_NAME" \
    --auth-type NONE \
    --region "$REGION" \
    --query FunctionUrl \
    --output text)

  # Allow public (unauthenticated) invocations via the Function URL
  aws lambda add-permission \
    --function-name "$FUNCTION_NAME" \
    --statement-id AllowPublicAccess \
    --action lambda:InvokeFunctionUrl \
    --principal "*" \
    --function-url-auth-type NONE \
    --region "$REGION" >/dev/null

  echo "Function URL: $FUNCTION_URL"
else
  FUNCTION_URL=$(aws lambda get-function-url-config \
    --function-name "$FUNCTION_NAME" \
    --region "$REGION" \
    --query FunctionUrl \
    --output text)
  echo "Function URL already exists: $FUNCTION_URL"
fi

# ── GitHub Actions OIDC deploy role ──────────────────────────────────────────

DEPLOY_ROLE_NAME="${FUNCTION_NAME}-github-deploy"
DEPLOY_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${DEPLOY_ROLE_NAME}"

OIDC_PROVIDER="token.actions.githubusercontent.com"
SUBJECT="repo:${GITHUB_ORG}/${GITHUB_REPO}:ref:refs/heads/main"

if ! aws iam get-role --role-name "$DEPLOY_ROLE_NAME" &>/dev/null; then
  echo "Creating GitHub Actions OIDC deploy role..."

  # Ensure the OIDC provider exists in this account
  OIDC_PROVIDER_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
  if ! aws iam get-open-id-connect-provider \
       --open-id-connect-provider-arn "$OIDC_PROVIDER_ARN" &>/dev/null; then
    echo "Creating OIDC provider for GitHub Actions..."
    aws iam create-open-id-connect-provider \
      --url "https://${OIDC_PROVIDER}" \
      --client-id-list "sts.amazonaws.com" \
      --thumbprint-list "6938fd4d98bab03faadb97b34396831e3780aea1" >/dev/null
  fi

  aws iam create-role \
    --role-name "$DEPLOY_ROLE_NAME" \
    --assume-role-policy-document "{
      \"Version\": \"2012-10-17\",
      \"Statement\": [{
        \"Effect\": \"Allow\",
        \"Principal\": {
          \"Federated\": \"${OIDC_PROVIDER_ARN}\"
        },
        \"Action\": \"sts:AssumeRoleWithWebIdentity\",
        \"Condition\": {
          \"StringEquals\": {
            \"${OIDC_PROVIDER}:aud\": \"sts.amazonaws.com\",
            \"${OIDC_PROVIDER}:sub\": \"${SUBJECT}\"
          }
        }
      }]
    }" >/dev/null

  aws iam put-role-policy \
    --role-name "$DEPLOY_ROLE_NAME" \
    --policy-name DeployPolicy \
    --policy-document "{
      \"Version\": \"2012-10-17\",
      \"Statement\": [{
        \"Effect\": \"Allow\",
        \"Action\": [
          \"lambda:UpdateFunctionCode\",
          \"lambda:GetFunction\"
        ],
        \"Resource\": \"arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${FUNCTION_NAME}\"
      }]
    }" >/dev/null

  echo "Deploy role created: $DEPLOY_ROLE_ARN"
else
  echo "Deploy role already exists — skipping."
fi

# ── Done ─────────────────────────────────────────────────────────────────────

echo ""
echo "Setup complete."
echo ""
echo "Next steps:"
echo "  1. Update the REPLACE_ME env vars on the Lambda function (console or CLI)."
echo "  2. Add this GitHub secret to the repo:"
echo "       Name:  AWS_DEPLOY_ROLE_ARN"
echo "       Value: ${DEPLOY_ROLE_ARN}"
echo "  3. Push to main — the CI pipeline will build and deploy automatically."
echo ""
echo "Function URL (use as NEXT_PUBLIC_API_URL in Vercel):"
echo "  ${FUNCTION_URL}"
