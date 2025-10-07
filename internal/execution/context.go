// Package execution discovers ECS execution context details.
package execution

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/benwsapp/aws-ssm-minimal/internal"
	"github.com/benwsapp/aws-ssm-minimal/internal/env"
	"github.com/benwsapp/aws-ssm-minimal/internal/metadata"
)

// Context captures region and identity information for the running task.
type Context struct {
	Region           string
	AvailabilityZone string
	TaskARN          string
}

// Provider discovers execution context information.
type Provider struct {
	MetadataProvider metadata.Provider
}

// NewProvider constructs a Provider instance.
func NewProvider(metadataProvider metadata.Provider) Provider {
	return Provider{MetadataProvider: metadataProvider}
}

// Discover returns the execution context for the current environment.
func (p Provider) Discover(ctx context.Context) (Context, error) {
	var execCtx Context
	p.populateFromMetadata(ctx, &execCtx)
	p.applyFallbacks(&execCtx)

	return p.ensureRegion(execCtx)
}

func (p Provider) populateFromMetadata(ctx context.Context, execCtx *Context) {
	metadataURI := env.GetString(internal.MetadataEnvKey)
	if metadataURI == "" {
		return
	}

	meta, err := p.MetadataProvider.FetchTaskMetadata(ctx, metadataURI)
	if err != nil {
		log.Printf("warning: failed to load ECS task metadata: %v", err)

		return
	}

	execCtx.AvailabilityZone = meta.AvailabilityZone
	execCtx.TaskARN = meta.TaskARN

	region, err := metadata.RegionFromTaskARN(meta.TaskARN)
	if err != nil {
		log.Printf("warning: unable to derive region from task ARN: %v", err)

		return
	}

	execCtx.Region = region
}

func (p Provider) applyFallbacks(execCtx *Context) {
	if execCtx.Region == "" {
		execCtx.Region = env.GetString(internal.EnvFallbackRegion)
	}

	if execCtx.Region == "" {
		execCtx.Region = env.GetString(internal.EnvFallbackDefaultRegion)
	}

	if execCtx.AvailabilityZone == "" {
		execCtx.AvailabilityZone = env.GetString(internal.EnvFallbackAvailabilityZone)
	}

	if execCtx.TaskARN == "" {
		execCtx.TaskARN = env.GetString(internal.EnvFallbackTaskARN)
	}
}

var errRegionNotFound = errors.New("execution region not found")

func (p Provider) ensureRegion(execCtx Context) (Context, error) {
	if execCtx.Region == "" {
		return Context{}, fmt.Errorf(
			"%w: set %s or %s",
			errRegionNotFound,
			internal.EnvFallbackRegion,
			internal.MetadataEnvKey,
		)
	}

	return execCtx, nil
}
