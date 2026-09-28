// lib/config.ts
// Central configuration for all stacks.
// Change values here — they propagate to every resource.

export const CONFIG = {
  // ── Project ───────────────────────────────────────────────────────────────
  projectName: "splitleger",
  region: "ap-southeast-1",

  // ── Domain ────────────────────────────────────────────────────────────────
  domainName: "ledger.kevinsanjula.me",
  apiSubdomain: "api.ledger.kevinsanjula.me",
  vpc: {
    cidr: "10.0.0.0/16",
    maxAzs: 2,
    // Lambda runs in isolated subnets. One NAT gateway provides outbound
    // access to Aiven and other managed services without public ENIs.
    natGateways: 1,
  },

  // ── Lambda ────────────────────────────────────────────────────────────────
  lambda: {
    memoryMiB: 512,
    timeoutSeconds: 30,
    reservedConcurrentExecutions: 20,
  },

  // ── ElastiCache (Redis) ───────────────────────────────────────────────────
  redis: {
    nodeType: "cache.t3.micro",
    engineVersion: "7.1",
    port: 6379,
  },

  // ── S3 ────────────────────────────────────────────────────────────────────
  s3: {
    receiptsBucketName: "splitleger-receipts-prod",
    presignExpirySeconds: 900,
    maxUploadSizeMB: 10,
  },

  // ── CloudWatch Logs ───────────────────────────────────────────────────────
  logs: {
    retentionDays: 7,
  },

  // ── Tags applied to every resource ───────────────────────────────────────
  tags: {
    Project: "SplitLedger",
    Environment: "production",
    ManagedBy: "CDK",
    Owner: "kevin",
  },
} as const;

export type Config = typeof CONFIG;
