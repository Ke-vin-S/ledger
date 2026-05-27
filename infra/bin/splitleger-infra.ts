#!/usr/bin/env node
// bin/splitleger-infra.ts
// CDK app entrypoint. Instantiates all stacks in dependency order.
//
// Deploy order (CDK handles this automatically via cross-stack references):
//   1. NetworkStack   — VPC, subnets, security groups
//   2. DataStack      — Redis, S3, SSM params, log group
//   3. AppStack       — ECR, ECS, ALB, ACM, Route 53
//   4. PipelineStack  — GitHub Actions IAM role
//
// Usage:
//   cdk deploy --all               # deploy everything
//   cdk deploy SplitlegerNetwork   # deploy a single stack
//   cdk diff                       # preview changes
//   cdk destroy --all              # tear down (careful in prod)

import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { NetworkStack }  from '../lib/network-stack';
import { DataStack }     from '../lib/data-stack';
import { AppStack }      from '../lib/app-stack';
import { PipelineStack } from '../lib/pipeline-stack';
import { CONFIG }        from '../lib/config';

const app = new cdk.App();

const env: cdk.Environment = {
  account: process.env.CDK_DEFAULT_ACCOUNT,
  region:  CONFIG.region,
};

// ── Stack 1: Network ──────────────────────────────────────────────────────
const networkStack = new NetworkStack(app, 'SplitlegerNetwork', {
  env,
  stackName:   'splitleger-network',
  description: 'SplitLedger VPC, subnets, and security groups',
  terminationProtection: true,   // prevent accidental deletion
});

// ── Stack 2: Data ─────────────────────────────────────────────────────────
const dataStack = new DataStack(app, 'SplitlegerData', {
  env,
  stackName:   'splitleger-data',
  description: 'SplitLedger ElastiCache Redis, S3, SSM parameters, CloudWatch log group',
  terminationProtection: true,
  vpc:     networkStack.vpc,
  redisSg: networkStack.redisSg,
});

// ── Stack 3: App ──────────────────────────────────────────────────────────
const appStack = new AppStack(app, 'SplitlegerApp', {
  env,
  stackName:   'splitleger-app',
  description: 'SplitLedger ECS Fargate service, ALB, ECR, ACM, Route 53',
  terminationProtection: true,
  vpc:             networkStack.vpc,
  albSg:           networkStack.albSg,
  ecsSg:           networkStack.ecsSg,
  receiptsBucket:  dataStack.receiptsBucket,
  ssmParamArns:    dataStack.ssmParamArns,
  redisEndpoint:   dataStack.redisEndpoint,
});

// ── Stack 4: Pipeline ─────────────────────────────────────────────────────
// IMPORTANT: Set githubRepo to your actual GitHub org/repo name.
// Format: 'your-github-username/splitleger-api'
new PipelineStack(app, 'SplitlegerPipeline', {
  env,
  stackName:   'splitleger-pipeline',
  description: 'SplitLedger GitHub Actions OIDC role for CI/CD',
  ecrRepository:  appStack.ecrRepository,
  ecsCluster:     appStack.ecsCluster,
  fargateService: appStack.fargateService,
  githubRepo:     'YOUR_GITHUB_USERNAME/splitleger-api',  // ← change this
});

app.synth();
