package ssmagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var errMissingActivation = errors.New("activation credentials not provided")

// Register invokes the amazon-ssm-agent binary to register with Systems Manager.
func Register(ctx context.Context, agentPath, region, activationID, activationCode string) error {
	if activationID == "" || activationCode == "" {
		return errMissingActivation
	}

	cmd := exec.CommandContext(
		ctx,
		agentPath,
		"-register",
		"-code", activationCode,
		"-id", activationID,
		"-region", region,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	runErr := cmd.Run()
	if runErr != nil {
		return fmt.Errorf("run amazon-ssm-agent registration: %w", runErr)
	}

	return nil
}
