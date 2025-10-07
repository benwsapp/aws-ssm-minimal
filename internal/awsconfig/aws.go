// Package awsconfig centralizes AWS SDK client construction helpers.
package awsconfig

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// NewSSMClient builds an SSM client for the specified region.
func NewSSMClient(ctx context.Context, region string) (*ssm.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return ssm.NewFromConfig(cfg), nil
}
