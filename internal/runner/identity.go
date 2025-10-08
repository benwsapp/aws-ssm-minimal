package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	runtimeconfig "github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

var errMissingManagedInstanceID = errors.New("registration file missing managed instance id")

const (
	runtimeConfigDir      = "/var/lib/amazon/ssm/runtimeconfig"
	runtimeIdentityConfig = "identity_config.json"
	registrationFile      = "/var/lib/amazon/ssm/registration"

	runtimeDirPerm  = 0o700
	runtimeFilePerm = 0o600
)

//nolint:tagliatelle // JSON keys must match SSM runtime schema exactly.
type runtimeIdentity struct {
	IdentityType            string `json:"IdentityType"`
	OnPremRegistrationType  string `json:"OnPremRegistrationType"`
	OnPremRegion            string `json:"OnPremRegion"`
	OnPremManagedInstanceID string `json:"OnPremManagedInstanceID"`
}

//nolint:tagliatelle // Matches SSM registration schema.
type registrationInfo struct {
	ManagedInstanceID string `json:"ManagedInstanceID"`
}

func persistIdentity(region string) error {
	payload, err := os.ReadFile(registrationFile)
	if err != nil {
		return fmt.Errorf("read registration file: %w", err)
	}

	var info registrationInfo

	err = json.Unmarshal(payload, &info)
	if err != nil {
		return fmt.Errorf("decode registration file: %w", err)
	}

	if info.ManagedInstanceID == "" {
		return errMissingManagedInstanceID
	}

	err = os.MkdirAll(runtimeConfigDir, runtimeDirPerm)
	if err != nil {
		return fmt.Errorf("create runtime config dir: %w", err)
	}

	identity := runtimeIdentity{
		IdentityType:            "OnPrem",
		OnPremRegistrationType:  "Managed",
		OnPremRegion:            region,
		OnPremManagedInstanceID: info.ManagedInstanceID,
	}

	data, marshalErr := json.Marshal(identity)
	if marshalErr != nil {
		return fmt.Errorf("marshal identity payload: %w", marshalErr)
	}

	target := filepath.Join(runtimeConfigDir, runtimeIdentityConfig)

	err = os.WriteFile(target, data, runtimeFilePerm)
	if err != nil {
		return fmt.Errorf("write runtime identity: %w", err)
	}

	runtimeClient := runtimeconfig.NewIdentityRuntimeConfigClient()
	configPayload := runtimeconfig.IdentityRuntimeConfig{
		SchemaVersion:          "1.1",
		InstanceId:             info.ManagedInstanceID,
		IdentityType:           "OnPrem",
		ShareFile:              filepath.Join(runtimeConfigDir, runtimeIdentityConfig),
		ShareProfile:           "",
		CredentialsExpiresAt:   time.Time{},
		CredentialsRetrievedAt: time.Time{},
		CredentialSource:       "",
	}

	err = runtimeClient.SaveConfig(configPayload)
	if err != nil {
		return fmt.Errorf("save runtime config via client: %w", err)
	}

	return nil
}
