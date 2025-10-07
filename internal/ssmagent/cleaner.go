// Package ssmagent provides utilities for interacting with amazon-ssm-agent.
package ssmagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const cleanupTimeout = 30 * time.Second

// Cleaner tears down activations and managed instance registrations.
type Cleaner struct {
	once sync.Once

	client           *ssm.Client
	activationID     string
	registrationPath string
}

// NewCleaner constructs a Cleaner tied to the provided activation metadata.
func NewCleaner(client *ssm.Client, activationID, registrationPath string) *Cleaner {
	return &Cleaner{
		once:             sync.Once{},
		client:           client,
		activationID:     activationID,
		registrationPath: registrationPath,
	}
}

// Cleanup removes the activation and managed instance registration, returning the first error encountered.
func (c *Cleaner) Cleanup(ctx context.Context) error {
	var cleanupErr error

	c.once.Do(func() {
		cleanupErr = c.runCleanup(ctx)
	})

	return cleanupErr
}

func (c *Cleaner) runCleanup(ctx context.Context) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	deleteErr := c.deleteActivation(cleanupCtx)
	if deleteErr != nil {
		return deleteErr
	}

	return c.deregisterInstance(cleanupCtx)
}

func (c *Cleaner) deleteActivation(ctx context.Context) error {
	if c.activationID == "" {
		return nil
	}

	input := &ssm.DeleteActivationInput{ActivationId: aws.String(c.activationID)}

	deleteResult, deleteErr := c.client.DeleteActivation(ctx, input)
	_ = deleteResult

	if deleteErr != nil {
		return fmt.Errorf("delete activation %s: %w", c.activationID, deleteErr)
	}

	log.Printf("deleted activation %s", c.activationID)

	return nil
}

func (c *Cleaner) deregisterInstance(ctx context.Context) error {
	instanceID, err := readManagedInstanceID(c.registrationPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read registration: %w", err)
	}

	if instanceID == "" {
		log.Printf("warning: managed instance ID was empty; skipping deregistration")

		return nil
	}

	input := &ssm.DeregisterManagedInstanceInput{InstanceId: aws.String(instanceID)}

	deregisterResult, deregisterErr := c.client.DeregisterManagedInstance(ctx, input)
	_ = deregisterResult

	if deregisterErr != nil {
		return fmt.Errorf("deregister instance %s: %w", instanceID, deregisterErr)
	}

	log.Printf("deregistered managed instance %s", instanceID)

	return nil
}

// registrationDocument matches the amazon-ssm-agent registration file structure.
type registrationDocument struct {
	ManagedInstanceID string `json:"ManagedInstanceID"` //nolint:tagliatelle // controlled by AWS agent
}

func readManagedInstanceID(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path validated before call
	if err != nil {
		return "", fmt.Errorf("read registration file: %w", err)
	}

	var doc registrationDocument

	unmarshalErr := json.Unmarshal(data, &doc)
	if unmarshalErr != nil {
		return "", fmt.Errorf("decode registration file: %w", unmarshalErr)
	}

	return strings.TrimSpace(doc.ManagedInstanceID), nil
}
