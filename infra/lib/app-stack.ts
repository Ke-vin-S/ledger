// lib/app-stack.ts
// Lambda Function URL, IAM permissions, encrypted runtime configuration, and S3 access.

import * as path    from 'node:path';
import * as cdk     from 'aws-cdk-lib';
import * as ec2     from 'aws-cdk-lib/aws-ec2';
import * as iam     from 'aws-cdk-lib/aws-iam';
import * as lambda  from 'aws-cdk-lib/aws-lambda';
import * as logs    from 'aws-cdk-lib/aws-logs';
import * as s3      from 'aws-cdk-lib/aws-s3';
import { Construct } from 'constructs';
import { CONFIG }   from './config';

interface AppStackProps extends cdk.StackProps {
  vpc:            ec2.Vpc;
  lambdaSg:       ec2.SecurityGroup;
  receiptsBucket: s3.Bucket;
  ssmParamArns:   string[];
}

export class AppStack extends cdk.Stack {
  public readonly apiFunction: lambda.Function;
  public readonly apiUrl: string;

  constructor(scope: Construct, id: string, props: AppStackProps) {
    super(scope, id, props);

    const { vpc, lambdaSg, receiptsBucket, ssmParamArns } = props;
    const code = lambda.Code.fromAsset(path.join(__dirname, '../../api'), {
      bundling: {
        image: cdk.DockerImage.fromRegistry('public.ecr.aws/docker/library/golang:1.25'),
        command: [
          'sh',
          '-c',
          'mkdir -p /asset-output /tmp/go-build /tmp/go-mod && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /asset-output/bootstrap ./cmd/api',
        ],
        outputType: cdk.BundlingOutput.NOT_ARCHIVED,
      },
    });

    const logGroup = new logs.LogGroup(this, 'ApiLogGroup', {
      logGroupName: `/aws/lambda/${CONFIG.projectName}-api`,
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

    this.apiFunction = new lambda.Function(this, 'ApiFunction', {
      functionName: `${CONFIG.projectName}-api`,
      description: 'SplitLedger Go API on Lambda Function URL',
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code,
      architecture: lambda.Architecture.X86_64,
      memorySize: CONFIG.lambda.memoryMiB,
      timeout: cdk.Duration.seconds(CONFIG.lambda.timeoutSeconds),
      reservedConcurrentExecutions: CONFIG.lambda.reservedConcurrentExecutions,
      vpc,
      vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_ISOLATED },
      securityGroups: [lambdaSg],
      logGroup,
      environment: {
        PORT: '8080',
        ENV: 'production',
        LOG_LEVEL: 'info',
        REGION: CONFIG.region,
        FRONTEND_URL: `https://${CONFIG.domainName}`,
        SSM_PARAMETER_PREFIX: `/${CONFIG.projectName}`,
      },
    });
    this.apiFunction.addToRolePolicy(new iam.PolicyStatement({
      sid:       'ReadEncryptedConfiguration',
      effect:    iam.Effect.ALLOW,
      actions:   ['ssm:GetParameter', 'ssm:GetParameters'],
      resources: ssmParamArns,
    }));
    this.apiFunction.addToRolePolicy(new iam.PolicyStatement({
      sid:       'DecryptSecureStrings',
      effect:    iam.Effect.ALLOW,
      actions:   ['kms:Decrypt'],
      resources: [`arn:aws:kms:${CONFIG.region}:${cdk.Stack.of(this).account}:alias/aws/ssm`],
    }));

    this.apiFunction.addToRolePolicy(new iam.PolicyStatement({
      sid:       'ReceiptObjectAccess',
      effect:    iam.Effect.ALLOW,
      actions:   ['s3:GetObject', 's3:PutObject', 's3:GetObjectAttributes'],
      resources: [receiptsBucket.bucketArn, `${receiptsBucket.bucketArn}/*`],
    }));
    this.apiFunction.addToRolePolicy(new iam.PolicyStatement({
      sid:       'SendTransactionalEmail',
      effect:    iam.Effect.ALLOW,
      actions:   ['ses:SendEmail', 'ses:SendRawEmail'],
      resources: ['*'],
    }));
    this.apiFunction.addToRolePolicy(new iam.PolicyStatement({
      sid:       'ReadBucketLocation',
      effect:    iam.Effect.ALLOW,
      actions:   ['s3:GetBucketLocation'],
      resources: [receiptsBucket.bucketArn],
    }));

    const functionUrl = this.apiFunction.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
      cors: {
        allowCredentials: true,
        allowedOrigins: [`https://${CONFIG.domainName}`, 'http://localhost:3000'],
        allowedHeaders: ['Authorization', 'Content-Type', 'X-Request-ID'],
        allowedMethods: [
          lambda.HttpMethod.GET,
          lambda.HttpMethod.POST,
          lambda.HttpMethod.PATCH,
          lambda.HttpMethod.DELETE,
          lambda.HttpMethod.OPTIONS,
        ],
        exposedHeaders: ['Set-Cookie', 'X-Request-ID'],
        maxAge: cdk.Duration.minutes(5),
      },
    });
    functionUrl.grantInvokeUrl(new iam.AnyPrincipal());
    this.apiUrl = functionUrl.url;

    new cdk.CfnOutput(this, 'ApiFunctionArn', {
      value: this.apiFunction.functionArn,
      exportName: 'SplitlegerApiFunctionArn',
    });
    new cdk.CfnOutput(this, 'ApiExecutionRoleArn', {
      value: this.apiFunction.role!.roleArn,
      exportName: 'SplitlegerApiExecutionRoleArn',
    });
    new cdk.CfnOutput(this, 'ApiFunctionUrl', {
      value: this.apiUrl,
      exportName: 'SplitlegerApiFunctionUrl',
      description: 'Set NEXT_PUBLIC_API_URL to this URL',
    });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
