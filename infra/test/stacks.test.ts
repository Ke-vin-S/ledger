import * as cdk from 'aws-cdk-lib';
import { Template, Match } from 'aws-cdk-lib/assertions';
import { NetworkStack } from '../lib/network-stack';
import { DataStack } from '../lib/data-stack';
import { AppStack } from '../lib/app-stack';
import { PipelineStack } from '../lib/pipeline-stack';
import { CONFIG } from '../lib/config';

const env = { account: '123456789012', region: CONFIG.region };

function synth() {
  const app = new cdk.App();
  const network = new NetworkStack(app, 'Network', { env });
  const data = new DataStack(app, 'Data', { env, vpc: network.vpc, redisSg: network.redisSg });
  const appStack = new AppStack(app, 'App', {
    env,
    vpc: network.vpc,
    lambdaSg: network.lambdaSg,
    receiptsBucket: data.receiptsBucket,
    ssmParamArns: data.ssmParamArns,
  });
  new PipelineStack(app, 'Pipeline', {
    env,
    lambdaFunction: appStack.apiFunction,
    githubRepo: 'Ke-vin-S/ledger',
  });
  return { network, data, appStack, app };
}

describe('NetworkStack', () => {
  const { network } = synth();
  const template = Template.fromStack(network);

  it('provisions the configured VPC', () => {
    template.hasResourceProperties('AWS::EC2::VPC', { CidrBlock: CONFIG.vpc.cidr });
  });

  it('provides one NAT gateway for Lambda egress', () => {
    template.resourceCountIs('AWS::EC2::NatGateway', 1);
  });

  it('keeps the S3 gateway endpoint free', () => {
    template.hasResourceProperties('AWS::EC2::VPCEndpoint', { VpcEndpointType: 'Gateway' });
  });

  it('creates only Lambda and Redis security groups', () => {
    template.resourceCountIs('AWS::EC2::SecurityGroup', 2);
  });
});

describe('DataStack', () => {
  const { data } = synth();
  const template = Template.fromStack(data);

  it('blocks all public access to receipts', () => {
    template.hasResourceProperties('AWS::S3::Bucket', {
      PublicAccessBlockConfiguration: {
        BlockPublicAcls: true,
        BlockPublicPolicy: true,
        IgnorePublicAcls: true,
        RestrictPublicBuckets: true,
      },
    });
  });

  it('creates Redis and encrypted configuration parameters', () => {
    template.resourceCountIs('AWS::ElastiCache::CacheCluster', 1);
    const parameters = template.findResources('AWS::SSM::Parameter');
    expect(Object.keys(parameters).length).toBeGreaterThanOrEqual(10);
    template.resourceCountIs('AWS::SSM::Parameter', 10);
    template.hasResourceProperties('AWS::SSM::Parameter', { Type: 'SecureString' });
    const allParameters = Object.values(template.findResources('AWS::SSM::Parameter'));
    expect(allParameters).toHaveLength(10);
    expect(allParameters.filter(parameter => parameter.Properties.Type === 'SecureString')).toHaveLength(8);
    expect(allParameters.filter(parameter => parameter.Properties.Type === 'String')).toHaveLength(2);
  });
});


describe('AppStack', () => {
  const { appStack } = synth();
  const template = Template.fromStack(appStack);
  it('creates a native Lambda Function URL without plaintext secrets', () => {
    template.hasResourceProperties('AWS::Lambda::Function', {
      FunctionName: `${CONFIG.projectName}-api`,
      Handler: 'bootstrap',
      Runtime: 'provided.al2023',
      Environment: Match.objectLike({ Variables: Match.objectLike({ ENV: 'production' }) }),
    });
    template.resourceCountIs('AWS::Lambda::Url', 1);
    const functions = template.findResources('AWS::Lambda::Function');
    const serialized = JSON.stringify(functions);
    expect(serialized).not.toContain('DATABASE_URL');
    expect(serialized).not.toContain('JWT_PRIVATE_KEY');
  });

  it('attaches the Lambda to private subnets and grants SSM/S3 access', () => {
    template.hasResourceProperties('AWS::IAM::Policy', {
      PolicyDocument: Match.objectLike({
        Statement: Match.arrayWith([
          Match.objectLike({ Sid: 'ReadEncryptedConfiguration', Action: Match.anyValue() }),
        ]),
      }),
    });
    template.hasResourceProperties('AWS::Lambda::Function', {
      VpcConfig: Match.objectLike({ SubnetIds: Match.anyValue(), SecurityGroupIds: Match.anyValue() }),
    });
  });
});

describe('PipelineStack', () => {
  const { appStack, app } = synth();
  const pipeline = new PipelineStack(app, 'PipelineForTest', {
    env,
    lambdaFunction: appStack.apiFunction,
    githubRepo: 'Ke-vin-S/ledger',
  });
  const template = Template.fromStack(pipeline);

  it('registers GitHub OIDC', () => {
    template.hasResourceProperties('Custom::AWSCDKOpenIdConnectProvider', {
      Url: 'https://token.actions.githubusercontent.com',
    });
  });

  it('trusts only the main branch of the configured repository', () => {
    template.hasResourceProperties('AWS::IAM::Role', {
      AssumeRolePolicyDocument: Match.objectLike({
        Statement: Match.arrayWith([
          Match.objectLike({
            Condition: Match.objectLike({
              StringEquals: Match.objectLike({
                'token.actions.githubusercontent.com:sub': 'repo:Ke-vin-S/ledger:ref:refs/heads/main',
              }),
            }),
          }),
        ]),
      }),
    });
  });
});
