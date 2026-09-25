package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type lambdaSecret struct {
	envName       string
	parameterName string
	required      bool
}

var lambdaSecrets = []lambdaSecret{
	{envName: "DATABASE_URL", parameterName: "db_url", required: true},
	{envName: "REDIS_URL", parameterName: "redis_url", required: true},
	{envName: "JWT_PRIVATE_KEY", parameterName: "jwt_private_key", required: true},
	{envName: "JWT_PUBLIC_KEY", parameterName: "jwt_public_key", required: true},
	{envName: "GOOGLE_CLIENT_ID", parameterName: "google_client_id", required: true},
	{envName: "GOOGLE_CLIENT_SECRET", parameterName: "google_client_secret", required: true},
	{envName: "AIVEN_CA_CERT", parameterName: "aiven_ca_cert", required: false},
	{envName: "S3_BUCKET", parameterName: "s3_bucket", required: true},
	{envName: "EMAIL_FROM", parameterName: "email_from", required: false},
}

// LoadLambdaSecrets hydrates the process environment from encrypted SSM
// parameters before the API configuration is validated. Local execution does
// not call AWS and continues to use environment variables directly.
func LoadLambdaSecrets(ctx context.Context) error {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" && os.Getenv("AWS_EXECUTION_ENV") == "" {
		return nil
	}

	region := getenv("AWS_REGION", "ap-southeast-1")
	prefix := getenv("SSM_PARAMETER_PREFIX", "/splitleger")
	client := ssm.NewFromConfig(aws.Config{Region: region})
	for _, secret := range lambdaSecrets {
		result, err := client.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(prefix + "/" + secret.parameterName),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			if secret.required {
				return fmt.Errorf("load Lambda secret %s: %w", secret.parameterName, err)
			}
			continue
		}
		value := aws.ToString(result.Parameter.Value)
		if strings.HasPrefix(value, "REPLACE_ME") {
			if secret.required {
				return fmt.Errorf("Lambda secret %s still contains REPLACE_ME", secret.parameterName)
			}
			continue
		}
		if err := os.Setenv(secret.envName, value); err != nil {
			return fmt.Errorf("set Lambda secret %s: %w", secret.envName, err)
		}
	}
	return nil
}
