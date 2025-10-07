// Package activation handles creation of Systems Manager activations for the wrapper.
package activation

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/benwsapp/aws-ssm-minimal/internal"
	"github.com/benwsapp/aws-ssm-minimal/internal/env"
	"github.com/benwsapp/aws-ssm-minimal/internal/execution"
)

const (
	extraTagDelimiter     = ","
	extraTagKeyValueParts = 2
	defaultTagCapacity    = 4
)

// Service creates SSM activations for the wrapped agent.
type Service struct {
	client *ssm.Client
}

// NewService returns a Service backed by the provided SSM client.
func NewService(client *ssm.Client) Service {
	return Service{client: client}
}

// Result captures the activation credentials issued by SSM.
type Result struct {
	ActivationID   string
	ActivationCode string
}

// Create provisions an activation using the supplied execution context and IAM role.
func (s Service) Create(ctx context.Context, roleName string, execCtx execution.Context) (Result, error) {
	var input ssm.CreateActivationInput

	limit := int32(1)
	input.IamRole = aws.String(roleName)
	input.RegistrationLimit = aws.Int32(limit)

	input.Description = aws.String(activationDescription(execCtx.TaskARN))
	input.Tags = buildTags(execCtx)

	if execCtx.TaskARN != "" {
		input.DefaultInstanceName = aws.String(execCtx.TaskARN)
	}

	output, err := s.client.CreateActivation(ctx, &input)
	if err != nil {
		return Result{}, fmt.Errorf("create activation: %w", err)
	}

	return Result{
		ActivationID:   aws.ToString(output.ActivationId),
		ActivationCode: aws.ToString(output.ActivationCode),
	}, nil
}

func activationDescription(taskARN string) string {
	if desc := strings.TrimSpace(env.GetString(internal.EnvActivationDescription)); desc != "" {
		return desc
	}

	if taskARN == "" {
		return ""
	}

	return "SSM agent sidecar for " + taskARN
}

func buildTags(execCtx execution.Context) []types.Tag {
	tags := make([]types.Tag, 0, defaultTagCapacity)
	if execCtx.AvailabilityZone != "" {
		tags = append(tags, makeTag("ECS_TASK_AVAILABILITY_ZONE", execCtx.AvailabilityZone))
	}

	if execCtx.TaskARN != "" {
		tags = append(tags, makeTag("ECS_TASK_ARN", execCtx.TaskARN))
	}

	tags = append(tags, makeTag(internal.FaultInjectionSidecarTagKey, internal.FaultInjectionSidecarTagValue))

	return append(tags, parseExtraTags(env.GetString(internal.EnvAdditionalActivationTags))...)
}

func parseExtraTags(raw string) []types.Tag {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	segments := strings.Split(trimmed, extraTagDelimiter)
	result := make([]types.Tag, 0, len(segments))

	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		parts := strings.SplitN(segment, "=", extraTagKeyValueParts)
		if len(parts) != extraTagKeyValueParts {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			continue
		}

		result = append(result, makeTag(key, value))
	}

	return result
}

func makeTag(key, value string) types.Tag {
	return types.Tag{Key: aws.String(key), Value: aws.String(value)}
}
