#!/usr/bin/env node
// bin/splitleger-infra.ts
// CDK app entrypoint. Instantiates Network → Data → App → Pipeline.

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

const networkStack = new NetworkStack(app, 'SplitlegerNetwork', {
  env,
  stackName:   'splitleger-network',
  description: 'SplitLedger VPC and private service networking',
  terminationProtection: true,
});

const dataStack = new DataStack(app, 'SplitlegerData', {
  env,
  stackName:   'splitleger-data',
  description: 'SplitLedger Redis, S3 receipts, and encrypted SSM parameters',
  terminationProtection: true,
  vpc:     networkStack.vpc,
  redisSg: networkStack.redisSg,
});

const appStack = new AppStack(app, 'SplitlegerApp', {
  env,
  stackName:   'splitleger-app',
  description: 'SplitLedger Lambda API and Function URL',
  terminationProtection: true,
  vpc:            networkStack.vpc,
  lambdaSg:       networkStack.lambdaSg,
  receiptsBucket: dataStack.receiptsBucket,
  ssmParamArns:   dataStack.ssmParamArns,
});

new PipelineStack(app, 'SplitlegerPipeline', {
  env,
  stackName:   'splitleger-pipeline',
  description: 'SplitLedger GitHub Actions OIDC deployment role',
  lambdaFunction: appStack.apiFunction,
  githubRepo:     'Ke-vin-S/ledger',
});

app.synth();
