// lib/pipeline-stack.ts
// GitHub Actions OIDC role for deploying the Lambda function.

import * as cdk    from 'aws-cdk-lib';
import * as iam    from 'aws-cdk-lib/aws-iam';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { Construct } from 'constructs';
import { CONFIG } from './config';

interface PipelineStackProps extends cdk.StackProps {
  lambdaFunction: lambda.Function;
  githubRepo:     string;
}

export class PipelineStack extends cdk.Stack {
  public readonly githubActionsRole: iam.Role;

  constructor(scope: Construct, id: string, props: PipelineStackProps) {
    super(scope, id, props);

    const { lambdaFunction, githubRepo } = props;
    const githubOidcProvider = new iam.OpenIdConnectProvider(this, 'GithubOidcProvider', {
      url:         'https://token.actions.githubusercontent.com',
      clientIds:   ['sts.amazonaws.com'],
      thumbprints: ['6938fd4d98bab03faadb97b34396831e3780aea1'],
    });

    this.githubActionsRole = new iam.Role(this, 'GithubActionsRole', {
      roleName:      `${CONFIG.projectName}-github-actions`,
      description:   'Assumed by the SplitLedger main branch through GitHub OIDC',
      assumedBy:     new iam.WebIdentityPrincipal(githubOidcProvider.openIdConnectProviderArn, {
        StringEquals: {
          'token.actions.githubusercontent.com:aud': 'sts.amazonaws.com',
          'token.actions.githubusercontent.com:sub': `repo:${githubRepo}:ref:refs/heads/main`,
        },
      }),
      maxSessionDuration: cdk.Duration.hours(1),
    });

    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:       'UpdateLambdaCode',
      effect:    iam.Effect.ALLOW,
      actions:   ['lambda:UpdateFunctionCode', 'lambda:GetFunction', 'lambda:GetFunctionConfiguration'],
      resources: [lambdaFunction.functionArn],
    }));
    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:       'PublishLambdaVersion',
      effect:    iam.Effect.ALLOW,
      actions:   ['lambda:PublishVersion'],
      resources: [lambdaFunction.functionArn],
    }));

    new cdk.CfnOutput(this, 'GithubActionsRoleArn', {
      value:      this.githubActionsRole.roleArn,
      exportName: 'SplitlegerGithubActionsRoleArn',
      description: 'Set this as AWS_DEPLOY_ROLE_ARN in GitHub Actions secrets',
    });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
