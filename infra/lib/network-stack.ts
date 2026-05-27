// lib/network-stack.ts
// Creates the VPC, subnets, and all security groups.
// Every other stack imports resources from here.

import * as cdk  from 'aws-cdk-lib';
import * as ec2  from 'aws-cdk-lib/aws-ec2';
import { Construct } from 'constructs';
import { CONFIG } from './config';

export class NetworkStack extends cdk.Stack {
  // Exported for use by DataStack and AppStack
  public readonly vpc:        ec2.Vpc;
  public readonly albSg:      ec2.SecurityGroup;
  public readonly ecsSg:      ec2.SecurityGroup;
  public readonly redisSg:    ec2.SecurityGroup;

  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // ── VPC ────────────────────────────────────────────────────────────────
    // No NAT Gateway (saves ~$32/month).
    // Layout:
    //   Public subnets  → ALB
    //   Public subnets  → ECS tasks (no public IP, reachable only via ALB SG)
    //   Isolated subnets → ElastiCache (no internet access needed)
    this.vpc = new ec2.Vpc(this, 'Vpc', {
      vpcName:    `${CONFIG.projectName}-vpc`,
      ipAddresses: ec2.IpAddresses.cidr(CONFIG.vpc.cidr),
      maxAzs:     CONFIG.vpc.maxAzs,
      natGateways: CONFIG.vpc.natGateways,   // 0 — cost saving

      subnetConfiguration: [
        {
          name:       'public',
          subnetType: ec2.SubnetType.PUBLIC,
          cidrMask:   24,
          // ALB and ECS tasks live here.
          // ECS tasks have no public IP — ALB SG is the only inbound path.
        },
        {
          name:       'isolated',
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          cidrMask:   24,
          // ElastiCache lives here. No internet access, no NAT needed.
        },
      ],
    });

    // ── Security Group: ALB ────────────────────────────────────────────────
    // Accepts HTTPS from the internet. HTTP redirects to HTTPS (listener rule).
    this.albSg = new ec2.SecurityGroup(this, 'AlbSg', {
      vpc:               this.vpc,
      securityGroupName: `${CONFIG.projectName}-alb-sg`,
      description:       'ALB: accept HTTPS/HTTP from internet',
      allowAllOutbound:  true,
    });
    this.albSg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(443), 'HTTPS from internet');
    this.albSg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80),  'HTTP from internet (→ redirect)');

    // ── Security Group: ECS Tasks ──────────────────────────────────────────
    // Accepts traffic only from ALB on the container port.
    // Outbound unrestricted: needs to reach Aiven (Postgres), ECR, SSM, S3.
    this.ecsSg = new ec2.SecurityGroup(this, 'EcsSg', {
      vpc:               this.vpc,
      securityGroupName: `${CONFIG.projectName}-ecs-sg`,
      description:       'ECS tasks: inbound from ALB only',
      allowAllOutbound:  true,   // outbound: Aiven, ECR pull, SSM, S3
    });
    this.ecsSg.addIngressRule(
      ec2.Peer.securityGroupId(this.albSg.securityGroupId),
      ec2.Port.tcp(CONFIG.ecs.containerPort),
      'From ALB only',
    );

    // ── Security Group: ElastiCache (Redis) ────────────────────────────────
    // Accepts Redis traffic only from ECS tasks.
    this.redisSg = new ec2.SecurityGroup(this, 'RedisSg', {
      vpc:               this.vpc,
      securityGroupName: `${CONFIG.projectName}-redis-sg`,
      description:       'Redis: inbound from ECS tasks only',
      allowAllOutbound:  false,
    });
    this.redisSg.addIngressRule(
      ec2.Peer.securityGroupId(this.ecsSg.securityGroupId),
      ec2.Port.tcp(CONFIG.redis.port),
      'From ECS tasks only',
    );

    // ── VPC Endpoints (cost-saving alternative to NAT) ─────────────────────
    // These allow ECS tasks in public subnets to reach AWS services
    // without routing through the internet. Saves NAT Gateway costs.
    // Note: Interface endpoints cost ~$7/month total but are cheaper than NAT.
    // Gateway endpoints (S3, DynamoDB) are FREE.

    // S3 Gateway endpoint — free, allows ECS to pull layers from ECR via S3
    new ec2.GatewayVpcEndpoint(this, 'S3Endpoint', {
      vpc:     this.vpc,
      service: ec2.GatewayVpcEndpointAwsService.S3,
    });

    // ECR API endpoint (interface) — needed for docker pull in private subnet
    // Skipping interface endpoints for now since tasks are in public subnets.
    // Add these if you move tasks to private subnets later:
    //   ec2.InterfaceVpcEndpointAwsService.ECR
    //   ec2.InterfaceVpcEndpointAwsService.ECR_DOCKER
    //   ec2.InterfaceVpcEndpointAwsService.SSM
    //   ec2.InterfaceVpcEndpointAwsService.CLOUDWATCH_LOGS

    // ── Outputs ────────────────────────────────────────────────────────────
    new cdk.CfnOutput(this, 'VpcId',    { value: this.vpc.vpcId,              exportName: 'SplitlegerVpcId' });
    new cdk.CfnOutput(this, 'AlbSgId',  { value: this.albSg.securityGroupId,  exportName: 'SplitlegerAlbSgId' });
    new cdk.CfnOutput(this, 'EcsSgId',  { value: this.ecsSg.securityGroupId,  exportName: 'SplitlegerEcsSgId' });
    new cdk.CfnOutput(this, 'RedisSgId',{ value: this.redisSg.securityGroupId,exportName: 'SplitlegerRedisSgId' });

    // Tag everything
    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
