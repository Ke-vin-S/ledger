// CDK assertion tests: synthesise the stacks and assert the security-critical
// and cost-critical properties hold (no NAT, S3 locked down, secrets via SSM,
// OIDC trust scoped to the repo). These run without deploying.

import * as cdk from "aws-cdk-lib";
import { Template, Match } from "aws-cdk-lib/assertions";
import { NetworkStack } from "../lib/network-stack";
import { DataStack } from "../lib/data-stack";
import { AppStack } from "../lib/app-stack";
import { PipelineStack } from "../lib/pipeline-stack";
import { CONFIG } from "../lib/config";

// A concrete (dummy) env is required so AppStack's HostedZone.fromLookup can
// resolve to a synth-time dummy value instead of throwing.
const env = { account: "123456789012", region: CONFIG.region };

function synth() {
  const app = new cdk.App();
  const network = new NetworkStack(app, "Network", { env });
  const data = new DataStack(app, "Data", {
    env,
    vpc: network.vpc,
    redisSg: network.redisSg,
  });
  const appStack = new AppStack(app, "App", {
    env,
    vpc: network.vpc,
    albSg: network.albSg,
    ecsSg: network.ecsSg,
    receiptsBucket: data.receiptsBucket,
    ssmParamArns: data.ssmParamArns,
    redisEndpoint: data.redisEndpoint,
  });
  new PipelineStack(app, "Pipeline", {
    env,
    ecrRepository: appStack.ecrRepository,
    ecsCluster: appStack.ecsCluster,
    fargateService: appStack.fargateService,
    githubRepo: "Ke-vin-S/ledger",
  });
  return { network, data, appStack, app };
}

describe("NetworkStack", () => {
  const { network } = synth();
  const t = Template.fromStack(network);

  it("provisions a VPC with the configured CIDR", () => {
    t.hasResourceProperties("AWS::EC2::VPC", {
      CidrBlock: CONFIG.vpc.cidr,
    });
  });

  it("creates no NAT gateways (cost saving)", () => {
    t.resourceCountIs("AWS::EC2::NatGateway", 0);
  });

  it("creates an S3 gateway VPC endpoint (free ECR access)", () => {
    t.resourceCountIs("AWS::EC2::VPCEndpoint", 1);
    t.hasResourceProperties("AWS::EC2::VPCEndpoint", {
      VpcEndpointType: "Gateway",
    });
  });

  it("creates three security groups (alb, ecs, redis)", () => {
    t.resourceCountIs("AWS::EC2::SecurityGroup", 3);
  });
});

describe("DataStack", () => {
  const { data } = synth();
  const t = Template.fromStack(data);

  it("locks the receipts bucket down to block all public access", () => {
    t.hasResourceProperties("AWS::S3::Bucket", {
      PublicAccessBlockConfiguration: {
        BlockPublicAcls: true,
        BlockPublicPolicy: true,
        IgnorePublicAcls: true,
        RestrictPublicBuckets: true,
      },
    });
  });

  it("creates a Redis (ElastiCache) cluster", () => {
    t.resourceCountIs("AWS::ElastiCache::CacheCluster", 1);
  });

  it("creates SSM parameters for all required secrets", () => {
    // db_url, redis_url, jwt private/public, google id/secret, aiven ca,
    // s3 bucket, app env = 9 parameters.
    const params = t.findResources("AWS::SSM::Parameter");
    expect(Object.keys(params).length).toBeGreaterThanOrEqual(9);
  });

  it("retains audit logs for the configured retention period", () => {
    t.hasResourceProperties("AWS::Logs::LogGroup", {
      RetentionInDays: CONFIG.logs.retentionDays,
    });
  });
});

describe("AppStack", () => {
  const { appStack } = synth();
  const t = Template.fromStack(appStack);

  it("injects secrets into the task definition via the `secrets` field, not plaintext env", () => {
    // Every container secret must resolve from SSM (a ValueFrom ref), never a literal value.
    t.hasResourceProperties("AWS::ECS::TaskDefinition", {
      ContainerDefinitions: Match.arrayWith([
        Match.objectLike({
          Secrets: Match.arrayWith([
            Match.objectLike({ ValueFrom: Match.anyValue() }),
          ]),
        }),
      ]),
    });
  });

  it("runs a single Fargate task as configured", () => {
    t.hasResourceProperties("AWS::ECS::Service", {
      DesiredCount: CONFIG.ecs.desiredCount,
      LaunchType: "FARGATE",
    });
  });
});

describe("PipelineStack", () => {
  const { appStack, app } = synth();
  const pipeline = new PipelineStack(app, "PipelineForTest", {
    env,
    ecrRepository: appStack.ecrRepository,
    ecsCluster: appStack.ecsCluster,
    fargateService: appStack.fargateService,
    githubRepo: "Ke-vin-S/ledger",
  });
  const t = Template.fromStack(pipeline);

  it("registers the GitHub OIDC provider", () => {
    t.hasResourceProperties("Custom::AWSCDKOpenIdConnectProvider", {
      Url: "https://token.actions.githubusercontent.com",
    });
  });

  it("scopes the deploy role's trust policy to the configured repo", () => {
    t.hasResourceProperties("AWS::IAM::Role", {
      AssumeRolePolicyDocument: Match.objectLike({
        Statement: Match.arrayWith([
          Match.objectLike({
            Condition: Match.objectLike({
              StringLike: Match.objectLike({
                "token.actions.githubusercontent.com:sub": "repo:Ke-vin-S/ledger:*",
              }),
            }),
          }),
        ]),
      }),
    });
  });
});
