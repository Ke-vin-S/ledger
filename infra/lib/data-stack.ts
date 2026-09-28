// lib/data-stack.ts
// ElastiCache Redis, S3 receipts bucket, and encrypted SSM parameters.

import * as cdk         from 'aws-cdk-lib';
import * as ec2         from 'aws-cdk-lib/aws-ec2';
import * as elasticache from 'aws-cdk-lib/aws-elasticache';
import * as s3          from 'aws-cdk-lib/aws-s3';
import * as ssm         from 'aws-cdk-lib/aws-ssm';
import { Construct }    from 'constructs';
import { CONFIG }       from './config';

interface DataStackProps extends cdk.StackProps {
  vpc:     ec2.Vpc;
  redisSg: ec2.SecurityGroup;
}

export class DataStack extends cdk.Stack {
  public readonly redisEndpoint:  string;
  public readonly redisPort:      number;
  public readonly receiptsBucket: s3.Bucket;
  public readonly ssmParamArns:   string[];

  constructor(scope: Construct, id: string, props: DataStackProps) {
    super(scope, id, props);

    const { vpc, redisSg } = props;
    const redisSubnetGroup = new elasticache.CfnSubnetGroup(this, 'RedisSubnetGroup', {
      description:          'Isolated subnets for ElastiCache Redis',
      subnetIds:            vpc.isolatedSubnets.map(subnet => subnet.subnetId),
      cacheSubnetGroupName: `${CONFIG.projectName}-redis-subnet-group`,
    });

    const redisCluster = new elasticache.CfnCacheCluster(this, 'Redis', {
      clusterName:           `${CONFIG.projectName}-redis`,
      cacheNodeType:         CONFIG.redis.nodeType,
      engine:                'redis',
      engineVersion:         CONFIG.redis.engineVersion,
      numCacheNodes:         1,
      port:                  CONFIG.redis.port,
      cacheSubnetGroupName:  redisSubnetGroup.cacheSubnetGroupName!,
      vpcSecurityGroupIds:   [redisSg.securityGroupId],
      autoMinorVersionUpgrade: true,
      snapshotRetentionLimit: 1,
    });
    redisCluster.addDependency(redisSubnetGroup);
    this.redisEndpoint = redisCluster.attrRedisEndpointAddress;
    this.redisPort = CONFIG.redis.port;

    this.receiptsBucket = new s3.Bucket(this, 'ReceiptsBucket', {
      bucketName:        CONFIG.s3.receiptsBucketName,
      versioned:         false,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption:        s3.BucketEncryption.S3_MANAGED,
      enforceSSL:        true,
      lifecycleRules: [{
        id:         'move-to-ia',
        enabled:    true,
        transitions: [{
          storageClass:    s3.StorageClass.INFREQUENT_ACCESS,
          transitionAfter: cdk.Duration.days(30),
        }],
      }],
      cors: [{
        allowedMethods: [s3.HttpMethods.PUT, s3.HttpMethods.GET],
        allowedOrigins: [`https://${CONFIG.domainName}`, 'http://localhost:3000'],
        allowedHeaders: ['*'],
        maxAge:         3000,
      }],
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

    const paramPrefix = `/${CONFIG.projectName}`;
    const parameterNames: string[] = [];
    const addParameter = (id: string, name: string, value: string, description: string, type: 'String' | 'SecureString') => {
      new ssm.CfnParameter(this, id, {
        name,
        type,
        value,
        description,
        tier: 'Standard',
      });
      parameterNames.push(name);
    };

    addParameter('ParamDbUrl', `${paramPrefix}/db_url`, 'REPLACE_ME_postgresql://user:pass@host:5432/db?sslmode=require', 'Aiven PostgreSQL connection string (TLS required)', 'SecureString');
    addParameter('ParamRedisUrl', `${paramPrefix}/redis_url`, `REPLACE_ME_redis://${redisCluster.attrRedisEndpointAddress}:${CONFIG.redis.port}`, 'ElastiCache Redis connection URL', 'SecureString');
    addParameter('ParamJwtPrivateKey', `${paramPrefix}/jwt_private_key`, 'REPLACE_ME_RSA_PRIVATE_KEY_PEM', 'RS256 JWT signing private key (PEM format)', 'SecureString');
    addParameter('ParamJwtPublicKey', `${paramPrefix}/jwt_public_key`, 'REPLACE_ME_RSA_PUBLIC_KEY_PEM', 'RS256 JWT verification public key (PEM format)', 'SecureString');
    addParameter('ParamGoogleClientId', `${paramPrefix}/google_client_id`, 'REPLACE_ME', 'Google OAuth client ID', 'SecureString');
    addParameter('ParamGoogleClientSecret', `${paramPrefix}/google_client_secret`, 'REPLACE_ME', 'Google OAuth client secret', 'SecureString');
    addParameter('ParamAivenCaCert', `${paramPrefix}/aiven_ca_cert`, 'REPLACE_ME_CA_CERT_PEM', 'Aiven CA certificate for TLS verification', 'SecureString');
    addParameter('ParamEmailFrom', `${paramPrefix}/email_from`, 'REPLACE_ME', 'Verified SES sender address', 'SecureString');
    addParameter('ParamS3Bucket', `${paramPrefix}/s3_bucket`, CONFIG.s3.receiptsBucketName, 'S3 receipts bucket name', 'String');
    addParameter('ParamAppEnv', `${paramPrefix}/app_env`, 'production', 'Application environment', 'String');
    this.ssmParamArns = parameterNames.map(name => this.formatArn({
      service: 'ssm',
      resource: 'parameter',
      resourceName: name.replace(/^\//, ''),
      arnFormat: cdk.ArnFormat.SLASH_RESOURCE_NAME,
    }));

    new cdk.CfnOutput(this, 'RedisEndpoint', {
      value: redisCluster.attrRedisEndpointAddress,
      exportName: 'SplitlegerRedisEndpoint',
    });
    new cdk.CfnOutput(this, 'ReceiptsBucketName', {
      value: this.receiptsBucket.bucketName,
      exportName: 'SplitlegerReceiptsBucket',
    });
    new cdk.CfnOutput(this, 'ReceiptsBucketArn', {
      value: this.receiptsBucket.bucketArn,
      exportName: 'SplitlegerReceiptsBucketArn',
    });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
