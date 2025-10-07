// Package internal defines shared configuration constants.
package internal

const (
	// DefaultTTLSeconds defines the default lifetime for the wrapped service.
	DefaultTTLSeconds = 3600

	// DefaultShutdownGraceSeconds defines the default graceful shutdown window.
	DefaultShutdownGraceSeconds = 15

	// RegistrationFilePath is the default location for the SSM registration file.
	RegistrationFilePath = "/var/lib/amazon/ssm/registration"

	// MetadataEnvKey identifies the ECS metadata URI.
	MetadataEnvKey = "ECS_CONTAINER_METADATA_URI_V4"

	// EnvManagedInstanceRole identifies the IAM role name for activation.
	EnvManagedInstanceRole = "MANAGED_INSTANCE_ROLE_NAME"

	// EnvTTLSeconds controls the TTL duration for the service.
	EnvTTLSeconds = "TTL_SECONDS"

	// EnvTTLShutdownGraceSeconds controls how long to wait after SIGTERM.
	EnvTTLShutdownGraceSeconds = "TTL_SHUTDOWN_GRACE_SECONDS"

	// EnvRegistrationFileOverride overrides the default SSM registration path.
	EnvRegistrationFileOverride = "SSM_REGISTRATION_FILE"

	// EnvFallbackAvailabilityZone provides the AZ when metadata is unavailable.
	EnvFallbackAvailabilityZone = "ECS_TASK_AVAILABILITY_ZONE"

	// EnvFallbackTaskARN provides the task ARN when metadata is unavailable.
	EnvFallbackTaskARN = "ECS_TASK_ARN"

	// EnvFallbackRegion provides the region when metadata is unavailable.
	EnvFallbackRegion = "AWS_REGION"

	// EnvFallbackDefaultRegion provides a secondary region fallback.
	EnvFallbackDefaultRegion = "AWS_DEFAULT_REGION"

	// EnvActivationDescription overrides the default activation description.
	EnvActivationDescription = "SSM_ACTIVATION_DESCRIPTION"

	// EnvAdditionalActivationTags supplies extra activation tags.
	EnvAdditionalActivationTags = "SSM_ACTIVATION_EXTRA_TAGS"

	// FaultInjectionSidecarTagKey identifies FIS sidecar activations.
	FaultInjectionSidecarTagKey = "FAULT_INJECTION_SIDECAR"

	// FaultInjectionSidecarTagValue is the FIS sidecar activation value.
	FaultInjectionSidecarTagValue = "true"
)
