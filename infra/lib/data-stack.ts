// lib/data-stack.ts
// ElastiCache (Redis), S3 receipts bucket, and SSM parameter placeholders.
// Postgres is on Aiven — no RDS resource here.

import * as cdk          from 'aws-cdk-lib';
import * as ec2          from 'aws-cdk-lib/aws-ec2';
import * as elasticache  from 'aws-cdk-lib/aws-elasticache';
import * as s3           from 'aws-cdk-lib/aws-s3';
import * as ssm          from 'aws-cdk-lib/aws-ssm';
import * as logs         from 'aws-cdk-lib/aws-logs';
import { Construct }     from 'constructs';
import { CONFIG }        from './config';

interface DataStackProps extends cdk.StackProps {
  vpc:     ec2.Vpc;
  redisSg: ec2.SecurityGroup;
}

export class DataStack extends cdk.Stack {
  public readonly redisEndpoint:     string;
  public readonly redisPort:         number;
  public readonly receiptsBucket:    s3.Bucket;

  // SSM parameter ARNs — passed to ECS task role for access
  public readonly ssmParamArns: string[];

  constructor(scope: Construct, id: string, props: DataStackProps) {
    super(scope, id, props);

    const { vpc, redisSg } = props;

    // ── ElastiCache Redis ──────────────────────────────────────────────────
    // Subnet group using isolated subnets — no internet access
    const redisSubnetGroup = new elasticache.CfnSubnetGroup(this, 'RedisSubnetGroup', {
      description:             'Isolated subnets for ElastiCache Redis',
      subnetIds:               vpc.isolatedSubnets.map(s => s.subnetId),
      cacheSubnetGroupName:    `${CONFIG.projectName}-redis-subnet-group`,
    });

    // Single-node Redis cluster (free tier: cache.t3.micro, 750 hrs/month for 12 months)
    // For HA in v2: switch to replicationGroup with 1 primary + 1 replica
    const redisCluster = new elasticache.CfnCacheCluster(this, 'Redis', {
      clusterName:           `${CONFIG.projectName}-redis`,
      cacheNodeType:         CONFIG.redis.nodeType,
      engine:                'redis',
      engineVersion:         CONFIG.redis.engineVersion,
      numCacheNodes:         1,
      port:                  CONFIG.redis.port,
      cacheSubnetGroupName:  redisSubnetGroup.cacheSubnetGroupName!,
      vpcSecurityGroupIds:   [redisSg.securityGroupId],

      // Automatic minor version upgrades — keep security patches applied
      autoMinorVersionUpgrade: true,

      // Snapshot for backup (free tier: 1 snapshot)
      snapshotRetentionLimit: 1,
    });
    redisCluster.addDependency(redisSubnetGroup);

    this.redisEndpoint = redisCluster.attrRedisEndpointAddress;
    this.redisPort     = CONFIG.redis.port;

    // ── S3: Receipts Bucket ────────────────────────────────────────────────
    this.receiptsBucket = new s3.Bucket(this, 'ReceiptsBucket', {
      bucketName:           CONFIG.s3.receiptsBucketName,
      versioned:            false,             // not needed for receipts
      blockPublicAccess:    s3.BlockPublicAccess.BLOCK_ALL,
      encryption:           s3.BucketEncryption.S3_MANAGED,
      enforceSSL:           true,

      // Lifecycle: move to Infrequent Access after 30 days (cost saving)
      lifecycleRules: [{
        id:         'move-to-ia',
        enabled:    true,
        transitions: [{
          storageClass:          s3.StorageClass.INFREQUENT_ACCESS,
          transitionAfter:       cdk.Duration.days(30),
        }],
      }],

      // CORS: allow pre-signed PUT uploads from the frontend origin
      cors: [{
        allowedMethods:  [s3.HttpMethods.PUT, s3.HttpMethods.GET],
        allowedOrigins:  [`https://${CONFIG.domainName}`, 'http://localhost:3000'],
        allowedHeaders:  ['*'],
        maxAge:          3000,
      }],

      removalPolicy:     cdk.RemovalPolicy.RETAIN,  // never auto-delete in prod
    });

    // ── SSM Parameter Store — placeholder parameters ───────────────────────
    // These are created with placeholder values.
    // IMPORTANT: After CDK deploy, manually update the SecureString values
    // via AWS Console or CLI before starting the ECS service.
    //
    // aws ssm put-parameter --name /splitleger/db_url --value "..." --type SecureString --overwrite

    const paramPrefix = `/${CONFIG.projectName}`;

    const ssmParams = [
      // Sensitive — SecureString
      new ssm.StringParameter(this, 'ParamDbUrl', {
        parameterName:  `${paramPrefix}/db_url`,
        stringValue:    'REPLACE_ME_postgresql://user:pass@host:5432/db?sslmode=require',
        description:    'Aiven PostgreSQL connection string (TLS required)',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamRedisUrl', {
        parameterName:  `${paramPrefix}/redis_url`,
        stringValue:    `REPLACE_ME_redis://${redisCluster.attrRedisEndpointAddress}:${CONFIG.redis.port}`,
        description:    'ElastiCache Redis connection URL',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamJwtPrivateKey', {
        parameterName:  `${paramPrefix}/jwt_private_key`,
        stringValue:    'REPLACE_ME_RSA_PRIVATE_KEY_PEM',
        description:    'RS256 JWT signing private key (PEM format)',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamJwtPublicKey', {
        parameterName:  `${paramPrefix}/jwt_public_key`,
        stringValue:    'REPLACE_ME_RSA_PUBLIC_KEY_PEM',
        description:    'RS256 JWT verification public key (PEM format)',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamGoogleClientId', {
        parameterName:  `${paramPrefix}/google_client_id`,
        stringValue:    'REPLACE_ME',
        description:    'Google OAuth client ID',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamGoogleClientSecret', {
        parameterName:  `${paramPrefix}/google_client_secret`,
        stringValue:    'REPLACE_ME',
        description:    'Google OAuth client secret',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamAivenCaCert', {
        parameterName:  `${paramPrefix}/aiven_ca_cert`,
        stringValue:    'REPLACE_ME_CA_CERT_PEM',
        description:    'Aiven CA certificate for TLS verification',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      // Non-sensitive — Standard String
      new ssm.StringParameter(this, 'ParamS3Bucket', {
        parameterName:  `${paramPrefix}/s3_bucket`,
        stringValue:    CONFIG.s3.receiptsBucketName,
        description:    'S3 receipts bucket name',
        tier:           ssm.ParameterTier.STANDARD,
      }),
      new ssm.StringParameter(this, 'ParamAppEnv', {
        parameterName:  `${paramPrefix}/app_env`,
        stringValue:    'production',
        description:    'Application environment',
        tier:           ssm.ParameterTier.STANDARD,
      }),
    ];

    this.ssmParamArns = ssmParams.map(p => p.parameterArn);

    // ── CloudWatch Log Group (API logs) ────────────────────────────────────
    new logs.LogGroup(this, 'ApiLogGroup', {
      logGroupName:  `/ecs/${CONFIG.projectName}-api`,
      retention:     logs.RetentionDays.ONE_WEEK,   // 7 days — stays in free tier
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    // ── Outputs ────────────────────────────────────────────────────────────
    new cdk.CfnOutput(this, 'RedisEndpoint', {
      value:      redisCluster.attrRedisEndpointAddress,
      exportName: 'SplitlegerRedisEndpoint',
    });
    new cdk.CfnOutput(this, 'ReceiptsBucketName', {
      value:      this.receiptsBucket.bucketName,
      exportName: 'SplitlegerReceiptsBucket',
    });
    new cdk.CfnOutput(this, 'ReceiptsBucketArn', {
      value:      this.receiptsBucket.bucketArn,
      exportName: 'SplitlegerReceiptsBucketArn',
    });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
