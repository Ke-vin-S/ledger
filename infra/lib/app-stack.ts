// lib/app-stack.ts
// ECR repository, ECS cluster + Fargate service, ALB, ACM certificate,
// Route 53 DNS records, and IAM roles.

import * as cdk          from 'aws-cdk-lib';
import * as ec2          from 'aws-cdk-lib/aws-ec2';
import * as ecr          from 'aws-cdk-lib/aws-ecr';
import * as ecs          from 'aws-cdk-lib/aws-ecs';
import * as iam          from 'aws-cdk-lib/aws-iam';
import * as elbv2        from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as acm          from 'aws-cdk-lib/aws-certificatemanager';
import * as route53      from 'aws-cdk-lib/aws-route53';
import * as route53targets from 'aws-cdk-lib/aws-route53-targets';
import * as s3           from 'aws-cdk-lib/aws-s3';
import { Construct }     from 'constructs';
import { CONFIG }        from './config';

interface AppStackProps extends cdk.StackProps {
  vpc:             ec2.Vpc;
  albSg:           ec2.SecurityGroup;
  ecsSg:           ec2.SecurityGroup;
  receiptsBucket:  s3.Bucket;
  ssmParamArns:    string[];
  redisEndpoint:   string;
}

export class AppStack extends cdk.Stack {
  public readonly ecrRepository:  ecr.Repository;
  public readonly ecsCluster:     ecs.Cluster;
  public readonly fargateService: ecs.FargateService;
  public readonly alb:            elbv2.ApplicationLoadBalancer;

  constructor(scope: Construct, id: string, props: AppStackProps) {
    super(scope, id, props);

    const { vpc, albSg, ecsSg, receiptsBucket, ssmParamArns } = props;

    // ── ECR Repository ─────────────────────────────────────────────────────
    this.ecrRepository = new ecr.Repository(this, 'ApiRepo', {
      repositoryName:   `${CONFIG.projectName}-api`,
      imageScanOnPush:  true,          // free vulnerability scanning

      // Keep only the last 10 images — free tier is 500 MB, Go images ~15 MB each
      lifecycleRules: [{
        maxImageCount:  10,
        description:    'Keep last 10 images',
        rulePriority:   1,
      }],

      removalPolicy: cdk.RemovalPolicy.RETAIN,  // don't delete images on stack delete
    });

    // ── IAM: ECS Task Execution Role ───────────────────────────────────────
    // Used by the ECS agent to pull images and write logs.
    // NOT the application role — this is infrastructure-level.
    const taskExecutionRole = new iam.Role(this, 'TaskExecutionRole', {
      roleName:    `${CONFIG.projectName}-task-execution-role`,
      assumedBy:   new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AmazonECSTaskExecutionRolePolicy'),
      ],
    });

    // Allow execution role to read SSM parameters (for secrets injection)
    taskExecutionRole.addToPolicy(new iam.PolicyStatement({
      sid:       'ReadSsmParameters',
      effect:    iam.Effect.ALLOW,
      actions:   ['ssm:GetParameters', 'ssm:GetParameter'],
      resources: ssmParamArns,
    }));

    // ── IAM: ECS Task Role ─────────────────────────────────────────────────
    // This is what the running application uses.
    // Principle of least privilege — only what the Go app actually needs.
    const taskRole = new iam.Role(this, 'TaskRole', {
      roleName:  `${CONFIG.projectName}-task-role`,
      assumedBy: new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
    });

    // S3: allow pre-signed URL generation and object operations
    taskRole.addToPolicy(new iam.PolicyStatement({
      sid:     'S3ReceiptsAccess',
      effect:  iam.Effect.ALLOW,
      actions: [
        's3:PutObject',
        's3:GetObject',
        's3:DeleteObject',
        's3:GetObjectAttributes',
      ],
      resources: [
        receiptsBucket.bucketArn,
        `${receiptsBucket.bucketArn}/*`,
      ],
    }));

    // S3: allow GeneratePresignedUrl (requires s3:PutObject on the bucket)
    taskRole.addToPolicy(new iam.PolicyStatement({
      sid:     'S3PresignedUrls',
      effect:  iam.Effect.ALLOW,
      actions: ['s3:GetBucketLocation'],
      resources: [receiptsBucket.bucketArn],
    }));

    // SSM: allow the app to read its own parameters at runtime
    // (separate from execution role — task role is for app-level access)
    taskRole.addToPolicy(new iam.PolicyStatement({
      sid:       'SsmReadParams',
      effect:    iam.Effect.ALLOW,
      actions:   ['ssm:GetParameter', 'ssm:GetParameters'],
      resources: ssmParamArns,
    }));

    // CloudWatch: allow structured log writes (belt-and-suspenders with awslogs driver)
    taskRole.addToPolicy(new iam.PolicyStatement({
      sid:     'CloudWatchLogs',
      effect:  iam.Effect.ALLOW,
      actions: [
        'logs:CreateLogStream',
        'logs:PutLogEvents',
        'logs:DescribeLogStreams',
      ],
      resources: [`arn:aws:logs:${CONFIG.region}:*:log-group:/ecs/${CONFIG.projectName}-api:*`],
    }));

    // ── ECS Cluster ────────────────────────────────────────────────────────
    this.ecsCluster = new ecs.Cluster(this, 'Cluster', {
      clusterName:        `${CONFIG.projectName}`,
      vpc,
      containerInsights:  false,    // saves ~$0.50/GB ingested; enable in v2 if needed
    });

    // ── ECS Task Definition ────────────────────────────────────────────────
    const taskDef = new ecs.FargateTaskDefinition(this, 'TaskDef', {
      family:           `${CONFIG.projectName}-api`,
      cpu:              CONFIG.ecs.cpu,
      memoryLimitMiB:   CONFIG.ecs.memoryMiB,
      executionRole:    taskExecutionRole,
      taskRole:         taskRole,
      runtimePlatform: {
        operatingSystemFamily: ecs.OperatingSystemFamily.LINUX,
        cpuArchitecture:       ecs.CpuArchitecture.X86_64,
      },
    });

    // ── Container Definition ───────────────────────────────────────────────
    const container = taskDef.addContainer('api', {
      image: ecs.ContainerImage.fromEcrRepository(
        this.ecrRepository,
        CONFIG.ecs.imageTag,
      ),
      containerName: 'api',
      essential:     true,

      // Non-sensitive environment variables
      environment: {
        PORT:    CONFIG.ecs.containerPort.toString(),
        ENV:     'production',
        REGION:  CONFIG.region,
      },

      // Sensitive values injected from SSM at task startup.
      // The container sees these as normal env vars.
      secrets: {
        DATABASE_URL:         ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('db_url')),
        REDIS_URL:            ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('redis_url')),
        JWT_PRIVATE_KEY:      ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('jwt_private_key')),
        JWT_PUBLIC_KEY:       ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('jwt_public_key')),
        GOOGLE_CLIENT_ID:     ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('google_client_id')),
        GOOGLE_CLIENT_SECRET: ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('google_client_secret')),
        AIVEN_CA_CERT:        ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('aiven_ca_cert')),
        S3_BUCKET:            ecs.Secret.fromSsmParameter(
          this.resolveStringParameter('s3_bucket')),
      },

      // Structured JSON logging to CloudWatch
      logging: ecs.LogDrivers.awsLogs({
        logGroup:         new (require('aws-cdk-lib/aws-logs').LogGroup)(
          this, 'ApiLogGroupRef', {
            logGroupName:  `/ecs/${CONFIG.projectName}-api`,
            retention:     require('aws-cdk-lib/aws-logs').RetentionDays.ONE_WEEK,
            removalPolicy: cdk.RemovalPolicy.DESTROY,
          }
        ),
        streamPrefix: 'api',
      }),

      // Health check — ALB also does its own health check
      healthCheck: {
        command:     ['CMD-SHELL', `curl -f http://localhost:${CONFIG.ecs.containerPort}${CONFIG.ecs.healthCheckPath} || exit 1`],
        interval:    cdk.Duration.seconds(30),
        timeout:     cdk.Duration.seconds(5),
        retries:     3,
        startPeriod: cdk.Duration.seconds(60),  // allow app startup time
      },

      portMappings: [{
        containerPort: CONFIG.ecs.containerPort,
        protocol:      ecs.Protocol.TCP,
      }],
    });

    // ── ALB ────────────────────────────────────────────────────────────────
    this.alb = new elbv2.ApplicationLoadBalancer(this, 'Alb', {
      loadBalancerName: `${CONFIG.projectName}-alb`,
      vpc,
      internetFacing:   true,
      securityGroup:    albSg,
      vpcSubnets:       { subnetType: ec2.SubnetType.PUBLIC },

      // Access logs add S3 costs — disable for personal project
      // Enable in v2 for production debugging
    });

    // ── ACM Certificate ────────────────────────────────────────────────────
    // Requires the Route 53 hosted zone to already exist.
    // DNS validation is automatic — CDK creates the CNAME records.
    const hostedZone = route53.HostedZone.fromLookup(this, 'HostedZone', {
      domainName: CONFIG.domainName,
    });

    const certificate = new acm.Certificate(this, 'ApiCertificate', {
      domainName:              CONFIG.apiSubdomain,
      validation:              acm.CertificateValidation.fromDns(hostedZone),
    });

    // ── ALB Listeners ──────────────────────────────────────────────────────
    // HTTP → HTTPS redirect
    this.alb.addListener('HttpListener', {
      port:         80,
      protocol:     elbv2.ApplicationProtocol.HTTP,
      defaultAction: elbv2.ListenerAction.redirect({
        protocol:   'HTTPS',
        port:       '443',
        permanent:  true,
      }),
    });

    // HTTPS → ECS target group
    const httpsListener = this.alb.addListener('HttpsListener', {
      port:         443,
      protocol:     elbv2.ApplicationProtocol.HTTPS,
      certificates: [certificate],
      sslPolicy:    elbv2.SslPolicy.TLS13_RES,  // TLS 1.2+ only
      defaultAction: elbv2.ListenerAction.fixedResponse(404, {
        contentType:  'application/json',
        messageBody:  '{"error":{"code":"NOT_FOUND","message":"Route not found"}}',
      }),
    });

    // ── ECS Fargate Service ────────────────────────────────────────────────
    this.fargateService = new ecs.FargateService(this, 'ApiService', {
      serviceName:     `${CONFIG.projectName}-api`,
      cluster:         this.ecsCluster,
      taskDefinition:  taskDef,
      desiredCount:    CONFIG.ecs.desiredCount,
      securityGroups:  [ecsSg],

      // Public subnet, no public IP → reachable only via ALB
      vpcSubnets: { subnetType: ec2.SubnetType.PUBLIC },
      assignPublicIp: false,

      // Rolling deployment: keep 100% running, spin up new task before stopping old
      minHealthyPercent: 100,
      maxHealthyPercent: 200,

      // Enable ECS Exec for production debugging (ssh into container)
      enableExecuteCommand: true,

      // Circuit breaker: auto-rollback if new deployment fails health checks
      circuitBreaker: { rollback: true },
    });

    // ── Target Group ───────────────────────────────────────────────────────
    const targetGroup = httpsListener.addTargets('ApiTargets', {
      targetGroupName: `${CONFIG.projectName}-api-tg`,
      port:            CONFIG.ecs.containerPort,
      protocol:        elbv2.ApplicationProtocol.HTTP,
      targets:         [this.fargateService],

      healthCheck: {
        path:                 CONFIG.ecs.healthCheckPath,
        healthyHttpCodes:     '200',
        interval:             cdk.Duration.seconds(30),
        timeout:              cdk.Duration.seconds(5),
        healthyThresholdCount:   2,
        unhealthyThresholdCount: 3,
      },

      // Deregistration delay: shorter for faster deploys
      deregistrationDelay: cdk.Duration.seconds(30),
    });

    // ── Route 53: api.splitleger.app → ALB ────────────────────────────────
    new route53.ARecord(this, 'ApiDnsRecord', {
      zone:       hostedZone,
      recordName: CONFIG.apiSubdomain,
      target:     route53.RecordTarget.fromAlias(
        new route53targets.LoadBalancerTarget(this.alb)
      ),
      ttl: cdk.Duration.minutes(5),
    });

    // ── Outputs ────────────────────────────────────────────────────────────
    new cdk.CfnOutput(this, 'EcrRepositoryUri', {
      value:      this.ecrRepository.repositoryUri,
      exportName: 'SplitlegerEcrUri',
      description: 'Use this URI in GitHub Actions to push images',
    });
    new cdk.CfnOutput(this, 'AlbDnsName', {
      value:      this.alb.loadBalancerDnsName,
      exportName: 'SplitlegerAlbDns',
    });
    new cdk.CfnOutput(this, 'EcsClusterName', {
      value:      this.ecsCluster.clusterName,
      exportName: 'SplitlegerClusterName',
    });
    new cdk.CfnOutput(this, 'EcsServiceName', {
      value:      this.fargateService.serviceName,
      exportName: 'SplitlegerServiceName',
    });
    new cdk.CfnOutput(this, 'ApiUrl', {
      value:      `https://${CONFIG.apiSubdomain}`,
      exportName: 'SplitlegerApiUrl',
      description: 'Public API base URL',
    });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }

  // Helper: resolve SSM parameter by name (avoids repeating the prefix)
  private resolveStringParameter(name: string) {
    const ssm = require('aws-cdk-lib/aws-ssm');
    return ssm.StringParameter.fromStringParameterName(
      this,
      `Ssm${name.replace(/_/g, '')}`,
      `/${CONFIG.projectName}/${name}`,
    );
  }
}
