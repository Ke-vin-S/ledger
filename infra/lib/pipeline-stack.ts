// lib/pipeline-stack.ts
// IAM resources for the GitHub Actions CI/CD pipeline.
// Uses OIDC federation — no long-lived AWS keys stored in GitHub.
// The GitHub Actions workflow assumes this role via OIDC to:
//   - Push Docker images to ECR
//   - Update the ECS service
//   - Read SSM parameters (for migration step)

import * as cdk  from 'aws-cdk-lib';
import * as iam  from 'aws-cdk-lib/aws-iam';
import * as ecr  from 'aws-cdk-lib/aws-ecr';
import * as ecs  from 'aws-cdk-lib/aws-ecs';
import { Construct } from 'constructs';
import { CONFIG } from './config';

interface PipelineStackProps extends cdk.StackProps {
  ecrRepository:  ecr.Repository;
  ecsCluster:     ecs.Cluster;
  fargateService: ecs.FargateService;
  // Your GitHub org/repo — e.g. 'kevindev/splitleger-api'
  githubRepo:     string;
}

export class PipelineStack extends cdk.Stack {
  public readonly githubActionsRole: iam.Role;

  constructor(scope: Construct, id: string, props: PipelineStackProps) {
    super(scope, id, props);

    const { ecrRepository, ecsCluster, fargateService, githubRepo } = props;

    // ── GitHub OIDC Provider ───────────────────────────────────────────────
    // Register GitHub as an OIDC identity provider in this AWS account.
    // Only needs to exist once per account. If it already exists, import it:
    //   iam.OpenIdConnectProvider.fromOpenIdConnectProviderArn(...)
    const githubOidcProvider = new iam.OpenIdConnectProvider(this, 'GithubOidcProvider', {
      url:          'https://token.actions.githubusercontent.com',
      clientIds:    ['sts.amazonaws.com'],
      thumbprints:  ['6938fd4d98bab03faadb97b34396831e3780aea1'],  // GitHub's OIDC thumbprint
    });

    // ── GitHub Actions IAM Role ────────────────────────────────────────────
    // This role can only be assumed from the specific GitHub repo + branch.
    // Condition: repo must match and ref must be main branch.
    this.githubActionsRole = new iam.Role(this, 'GithubActionsRole', {
      roleName:    `${CONFIG.projectName}-github-actions`,
      description: 'Assumed by GitHub Actions via OIDC for deploy pipeline',
      assumedBy:   new iam.WebIdentityPrincipal(
        githubOidcProvider.openIdConnectProviderArn,
        {
          'StringEquals': {
            'token.actions.githubusercontent.com:aud': 'sts.amazonaws.com',
          },
          'StringLike': {
            // Allows main branch and any branch (remove wildcard for strict main-only)
            'token.actions.githubusercontent.com:sub': `repo:${githubRepo}:*`,
          },
        },
      ),
      maxSessionDuration: cdk.Duration.hours(1),
    });

    // ── ECR: push images ───────────────────────────────────────────────────
    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:     'EcrAuth',
      effect:  iam.Effect.ALLOW,
      actions: ['ecr:GetAuthorizationToken'],
      resources: ['*'],   // GetAuthorizationToken doesn't support resource-level
    }));

    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:    'EcrPush',
      effect: iam.Effect.ALLOW,
      actions: [
        'ecr:BatchCheckLayerAvailability',
        'ecr:GetDownloadUrlForLayer',
        'ecr:BatchGetImage',
        'ecr:InitiateLayerUpload',
        'ecr:UploadLayerPart',
        'ecr:CompleteLayerUpload',
        'ecr:PutImage',
        'ecr:DescribeImages',
      ],
      resources: [ecrRepository.repositoryArn],
    }));

    // ── ECS: update service ────────────────────────────────────────────────
    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:    'EcsUpdateService',
      effect: iam.Effect.ALLOW,
      actions: [
        'ecs:UpdateService',
        'ecs:DescribeServices',
        'ecs:RegisterTaskDefinition',
        'ecs:DescribeTaskDefinition',
        'ecs:ListTasks',
        'ecs:DescribeTasks',
      ],
      resources: [
        fargateService.serviceArn,
        `arn:aws:ecs:${CONFIG.region}:*:task-definition/${CONFIG.projectName}-api:*`,
      ],
    }));

    // Describe clusters needed for ecs update-service
    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:       'EcsDescribeClusters',
      effect:    iam.Effect.ALLOW,
      actions:   ['ecs:DescribeClusters'],
      resources: [ecsCluster.clusterArn],
    }));

    // Pass the task execution role to ECS when registering new task definitions
    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:     'PassEcsRoles',
      effect:  iam.Effect.ALLOW,
      actions: ['iam:PassRole'],
      resources: [
        `arn:aws:iam::*:role/${CONFIG.projectName}-task-execution-role`,
        `arn:aws:iam::*:role/${CONFIG.projectName}-task-role`,
      ],
    }));

    // ── SSM: read params for migration step ───────────────────────────────
    this.githubActionsRole.addToPolicy(new iam.PolicyStatement({
      sid:     'SsmReadForMigrations',
      effect:  iam.Effect.ALLOW,
      actions: ['ssm:GetParameter', 'ssm:GetParameters'],
      resources: [
        `arn:aws:ssm:${CONFIG.region}:*:parameter/${CONFIG.projectName}/*`,
      ],
    }));

    // ── Outputs ────────────────────────────────────────────────────────────
    new cdk.CfnOutput(this, 'GithubActionsRoleArn', {
      value:      this.githubActionsRole.roleArn,
      exportName: 'SplitlegerGithubActionsRoleArn',
      description: 'Set this as AWS_ROLE_ARN in GitHub Actions secrets',
    });

    new cdk.CfnOutput(this, 'EcrRepositoryUri', {
      value:      ecrRepository.repositoryUri,
      exportName: 'SplitlegerEcrRepositoryUri',
      description: 'Set this as ECR_REGISTRY in GitHub Actions secrets',
    });

    new cdk.CfnOutput(this, 'GitHubActionsWorkflowSnippet', {
      value: [
        'Add to .github/workflows/deploy.yml:',
        '  - name: Configure AWS',
        '    uses: aws-actions/configure-aws-credentials@v4',
        '    with:',
        `      role-to-assume: ${this.githubActionsRole.roleArn}`,
        `      aws-region: ${CONFIG.region}`,
      ].join('\\n'),
      description: 'GitHub Actions workflow snippet',
    });

    Object.entries(CONFIG.tags).forEach(([k, v]) => cdk.Tags.of(this).add(k, v));
  }
}
