// lib/config.ts
// Central configuration for all stacks.
// Change values here — they propagate to every resource.

export const CONFIG = {
  // ── Project ───────────────────────────────────────────────────────────────
  projectName: 'splitleger',
  region:      'ap-southeast-1',   // Singapore

  // ── Domain ────────────────────────────────────────────────────────────────
  // Replace with your actual domain.
  // Route 53 hosted zone must already exist for this domain.
  domainName:       'splitleger.app',
  apiSubdomain:     'api.splitleger.app',

  // ── VPC ───────────────────────────────────────────────────────────────────
  vpc: {
    cidr:         '10.0.0.0/16',
    maxAzs:       2,               // 2 AZs — enough for ALB requirement + HA
    // No NAT Gateway to save ~$32/month.
    // ECS tasks go in public subnets with no public IP assigned.
    // They reach the internet via the IGW for outbound (Aiven, etc.)
    // but are unreachable inbound — ALB is the only ingress.
    natGateways:  0,
  },

  // ── ECS ───────────────────────────────────────────────────────────────────
  ecs: {
    cpu:          256,             // 0.25 vCPU
    memoryMiB:    512,
    containerPort: 8080,
    healthCheckPath: '/health',
    desiredCount: 1,               // scale up manually when needed
    // Docker image tag strategy: deploy with git SHA, keep 'latest' as fallback
    imageTag:     'latest',
  },

  // ── ElastiCache (Redis) ───────────────────────────────────────────────────
  redis: {
    nodeType:     'cache.t3.micro',  // free tier eligible (first 12 months)
    engineVersion: '7.1',
    port:          6379,
  },

  // ── S3 ────────────────────────────────────────────────────────────────────
  s3: {
    receiptsBucketName: 'splitleger-receipts-prod',
    // Pre-signed URL expiry (seconds) — enforced at bucket policy level
    presignExpirySeconds: 900,     // 15 min
    // Max object size enforced via bucket policy condition
    maxUploadSizeMB: 10,
  },

  // ── CloudWatch Logs ───────────────────────────────────────────────────────
  logs: {
    retentionDays: 7,              // keep costs at zero on free tier
  },

  // ── Tags applied to every resource ───────────────────────────────────────
  tags: {
    Project:     'SplitLedger',
    Environment: 'production',
    ManagedBy:   'CDK',
    Owner:       'kevin',
  },
} as const;

export type Config = typeof CONFIG;
