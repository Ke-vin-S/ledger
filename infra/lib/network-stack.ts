// lib/network-stack.ts
// VPC, subnets, and security groups for the Lambda API and private Redis.

import * as cdk  from 'aws-cdk-lib';
import * as ec2  from 'aws-cdk-lib/aws-ec2';
import { Construct } from 'constructs';
import { CONFIG } from './config';

export class NetworkStack extends cdk.Stack {
  public readonly vpc:       ec2.Vpc;
  public readonly lambdaSg: ec2.SecurityGroup;
  public readonly redisSg:  ec2.SecurityGroup;

  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    this.vpc = new ec2.Vpc(this, 'Vpc', {
      vpcName:    `${CONFIG.projectName}-vpc`,
      ipAddresses: ec2.IpAddresses.cidr(CONFIG.vpc.cidr),
      maxAzs:     CONFIG.vpc.maxAzs,
      natGateways: CONFIG.vpc.natGateways,
      subnetConfiguration: [
        {
          name:       'public',
          subnetType: ec2.SubnetType.PUBLIC,
          cidrMask:   24,
        },
        {
          name:       'isolated',
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          cidrMask:   24,
        },
      ],
    });

    this.lambdaSg = new ec2.SecurityGroup(this, 'LambdaSg', {
      vpc:               this.vpc,
      securityGroupName: `${CONFIG.projectName}-lambda-sg`,
      description:       'Lambda API: outbound access to managed services and Redis',
      allowAllOutbound:  true,
    });

    this.redisSg = new ec2.SecurityGroup(this, 'RedisSg', {
      vpc:               this.vpc,
      securityGroupName: `${CONFIG.projectName}-redis-sg`,
      description:       'Redis: inbound from the Lambda API only',
      allowAllOutbound:  false,
    });
    this.redisSg.addIngressRule(
      ec2.Peer.securityGroupId(this.lambdaSg.securityGroupId),
      ec2.Port.tcp(CONFIG.redis.port),
      'From Lambda API only',
    );

    new ec2.GatewayVpcEndpoint(this, 'S3Endpoint', {
      vpc:     this.vpc,
      service: ec2.GatewayVpcEndpointAwsService.S3,
    });

    new cdk.CfnOutput(this, 'VpcId',     { value: this.vpc.vpcId, exportName: 'SplitlegerVpcId' });
    new cdk.CfnOutput(this, 'LambdaSgId', { value: this.lambdaSg.securityGroupId, exportName: 'SplitlegerLambdaSgId' });
    new cdk.CfnOutput(this, 'RedisSgId', { value: this.redisSg.securityGroupId, exportName: 'SplitlegerRedisSgId' });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
