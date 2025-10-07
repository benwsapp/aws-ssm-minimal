// Package metadata fetches ECS task metadata required for registration.
package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	metadataPathSuffix = "/task"
	statusBodyLimit    = 4_096
	arnRegionIndex     = 3
)

var (
	errMetadataRequest    = errors.New("request ECS task metadata")
	errMetadataHTTPStatus = errors.New("unexpected ECS metadata status")
	errMetadataDecode     = errors.New("decode ECS metadata response")
	errMetadataRegion     = errors.New("invalid task ARN region")
)

// TaskMetadata represents the subset of ECS metadata used for registration.
type TaskMetadata struct {
	AvailabilityZone string `json:"AvailabilityZone"` //nolint:tagliatelle // AWS metadata casing
	TaskARN          string `json:"TaskARN"`          //nolint:tagliatelle // AWS metadata casing
}

// Provider retrieves ECS task metadata.
type Provider struct {
	Client HTTPClient
}

// HTTPClient abstracts http.Client for testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewProvider constructs a Provider instance.
func NewProvider(client HTTPClient) Provider {
	if client == nil {
		client = http.DefaultClient
	}

	return Provider{Client: client}
}

// FetchTaskMetadata retrieves metadata describing the running ECS task.
func (p Provider) FetchTaskMetadata(ctx context.Context, baseURI string) (TaskMetadata, error) {
	cleanBase := strings.TrimSuffix(baseURI, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanBase+metadataPathSuffix, nil)
	if err != nil {
		return TaskMetadata{}, fmt.Errorf("%w: %w", errMetadataRequest, err)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return TaskMetadata{}, fmt.Errorf("%w: %w", errMetadataRequest, err)
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("warning: failed to close metadata response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, statusBodyLimit))
		if readErr != nil {
			return TaskMetadata{}, fmt.Errorf("%w: status %d read body: %w", errMetadataHTTPStatus, resp.StatusCode, readErr)
		}

		return TaskMetadata{}, fmt.Errorf("%w: status %d body %q", errMetadataHTTPStatus, resp.StatusCode, string(body))
	}

	var meta TaskMetadata

	decodeErr := json.NewDecoder(resp.Body).Decode(&meta)
	if decodeErr != nil {
		return TaskMetadata{}, fmt.Errorf("%w: %w", errMetadataDecode, decodeErr)
	}

	return meta, nil
}

// RegionFromTaskARN extracts the AWS region component from a task ARN.
func RegionFromTaskARN(taskARN string) (string, error) {
	parts := strings.Split(taskARN, ":")
	if len(parts) <= arnRegionIndex {
		return "", fmt.Errorf("%w: %s", errMetadataRegion, taskARN)
	}

	region := strings.TrimSpace(parts[arnRegionIndex])
	if region == "" {
		return "", fmt.Errorf("%w: empty component in %s", errMetadataRegion, taskARN)
	}

	return region, nil
}
