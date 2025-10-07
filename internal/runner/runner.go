// Package runner orchestrates the TTL wrapper lifecycle.
package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/benwsapp/aws-ssm-minimal/internal"
	"github.com/benwsapp/aws-ssm-minimal/internal/activation"
	"github.com/benwsapp/aws-ssm-minimal/internal/awsconfig"
	"github.com/benwsapp/aws-ssm-minimal/internal/env"
	"github.com/benwsapp/aws-ssm-minimal/internal/execution"
	"github.com/benwsapp/aws-ssm-minimal/internal/metadata"
	"github.com/benwsapp/aws-ssm-minimal/internal/ssmagent"
	"github.com/benwsapp/aws-ssm-minimal/internal/supervisor"
)

const (
	defaultMetadataTimeout   = 5 * time.Second
	activationTimeout        = 30 * time.Second
	registrationTimeout      = 60 * time.Second
	metadataRegistrationBase = "/var/lib/amazon/ssm"
)

var (
	errArgsMissing             = errors.New("service command not specified")
	errNonPositiveTTL          = errors.New("ttl must be greater than zero")
	errRegistrationPathInvalid = errors.New("registration path outside allowed base")
)

// App represents the command-line entrypoint.
type App struct{}

// NewApp returns a new App instance.
func NewApp() App {
	return App{}
}

// Run executes the TTL runner and returns the exit code.
func (App) Run() (int, error) {
	args, err := parseArgs()
	if err != nil {
		return 1, err
	}

	ttl, grace, err := readDurations()
	if err != nil {
		return 1, err
	}

	roleName, err := env.MustGetNonEmpty(internal.EnvManagedInstanceRole)
	if err != nil {
		return 1, fmt.Errorf("read managed instance role: %w", err)
	}

	ctx := context.Background()

	execCtx, err := discoverExecutionContext(ctx)
	if err != nil {
		return 1, err
	}

	ssmClient, err := awsconfig.NewSSMClient(ctx, execCtx.Region)
	if err != nil {
		return 1, fmt.Errorf("create ssm client: %w", err)
	}

	registrationPath, err := resolveRegistrationPath()
	if err != nil {
		return 1, err
	}

	activationResult, cleanup, err := activateInstance(ctx, ssmClient, roleName, execCtx, registrationPath)
	if err != nil {
		return 1, err
	}
	defer cleanup()

	registrationErr := registerAgent(ctx, activationResult, execCtx.Region, args[0])
	if registrationErr != nil {
		return 1, registrationErr
	}

	return superviseCommand(ctx, args, ttl, grace)
}

func parseArgs() ([]string, error) {
	args := os.Args[1:]
	if len(args) == 0 {
		return nil, errArgsMissing
	}

	return args, nil
}

func readDurations() (time.Duration, time.Duration, error) {
	ttl, err := env.DurationSeconds(internal.EnvTTLSeconds, internal.DefaultTTLSeconds)
	if err != nil {
		return 0, 0, fmt.Errorf("read ttl seconds: %w", err)
	}

	if ttl <= 0 {
		return 0, 0, fmt.Errorf("%w: %s", errNonPositiveTTL, internal.EnvTTLSeconds)
	}

	grace, err := env.DurationSeconds(internal.EnvTTLShutdownGraceSeconds, internal.DefaultShutdownGraceSeconds)
	if err != nil {
		return 0, 0, fmt.Errorf("read shutdown grace: %w", err)
	}

	if grace < 0 {
		grace = 0
	}

	return ttl, grace, nil
}

func discoverExecutionContext(parent context.Context) (execution.Context, error) {
	ctx, cancel := context.WithTimeout(parent, defaultMetadataTimeout)
	defer cancel()

	provider := execution.NewProvider(metadata.NewProvider(nil))

	execCtx, err := provider.Discover(ctx)
	if err != nil {
		return execution.Context{}, fmt.Errorf("discover execution context: %w", err)
	}

	log.Printf("discovered execution context: region=%s az=%s taskArn=%s",
		execCtx.Region, execCtx.AvailabilityZone, execCtx.TaskARN)

	return execCtx, nil
}

// resolveRegistrationPath determines where the amazon-ssm-agent registration file lives.
func resolveRegistrationPath() (string, error) {
	path := env.GetString(internal.EnvRegistrationFileOverride)
	if path == "" {
		return internal.RegistrationFilePath, nil
	}

	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, metadataRegistrationBase) {
		return "", fmt.Errorf("%w: %s within %s", errRegistrationPathInvalid, clean, metadataRegistrationBase)
	}

	return clean, nil
}

func activateInstance(
	parent context.Context,
	client *ssm.Client,
	roleName string,
	execCtx execution.Context,
	registrationPath string,
) (activation.Result, func(), error) {
	ctx, cancel := context.WithTimeout(parent, activationTimeout)
	result, err := activation.NewService(client).Create(ctx, roleName, execCtx)

	cancel()

	if err != nil {
		return activation.Result{}, nil, fmt.Errorf("create activation: %w", err)
	}

	log.Printf("created SSM activation id=%s", result.ActivationID)

	cleaner := ssmagent.NewCleaner(client, result.ActivationID, registrationPath)
	cleanupFn := func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(parent, activationTimeout)
		defer cancelCleanup()

		cleanupErr := cleaner.Cleanup(cleanupCtx)
		if cleanupErr != nil {
			log.Printf("warning: cleanup failed: %v", cleanupErr)
		}
	}

	return result, cleanupFn, nil
}

func registerAgent(parent context.Context, activationResult activation.Result, region, agentPath string) error {
	ctx, cancel := context.WithTimeout(parent, registrationTimeout)
	defer cancel()

	registrationErr := ssmagent.Register(
		ctx,
		agentPath,
		region,
		activationResult.ActivationID,
		activationResult.ActivationCode,
	)
	if registrationErr != nil {
		return fmt.Errorf("register SSM agent: %w", registrationErr)
	}

	log.Printf("registered amazon-ssm-agent with activation id=%s", activationResult.ActivationID)

	return nil
}

func superviseCommand(ctx context.Context, args []string, ttl, grace time.Duration) (int, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- command path managed by container image
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	result, err := supervisor.Run(cmd, ttl, grace)
	if err != nil {
		return 1, fmt.Errorf("supervise service: %w", err)
	}

	if result.TTLExpired {
		log.Printf("ttl elapsed; exiting wrapper with status 0")

		return 0, nil
	}

	return result.ExitCode, nil
}
