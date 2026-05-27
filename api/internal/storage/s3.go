package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Presigner generates pre-signed PUT URLs for the configured bucket.
type S3Presigner struct {
	client *s3.PresignClient
	bucket string
}

// NewS3Presigner loads AWS config from the environment and constructs a presigner.
func NewS3Presigner(ctx context.Context, bucket, region string) (*S3Presigner, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	s3Client := s3.NewFromConfig(cfg)
	return &S3Presigner{
		client: s3.NewPresignClient(s3Client),
		bucket: bucket,
	}, nil
}

// PresignPut returns a pre-signed PUT URL valid for ttl.
func (p *S3Presigner) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	req, err := p.client.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return req.URL, nil
}
