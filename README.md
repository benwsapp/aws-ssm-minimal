> [!WARNING]
> Project is still in progress. This project is designed to be coupled with another project (not yet public) that will launch an SSM session to access private EKS clusters.

# aws-ssm-minimal

## Overview

`aws-ssm-minimal` is a purpose-built container image for running the AWS Systems Manager (SSM) agent as a sidecar in compute environments that do not ship with SSM pre-installed (for example, ECS Fargate tasks, EKS Pods, or plain OCI runtimes). The image bundles:

* A lightweight TTL wrapper written in Go that supervises the agent, adds a configurable time-to-live, forwards signals, and performs activation clean-up when the container exits.
* A non-root build of the official [`aws/amazon-ssm-agent`](https://github.com/aws/amazon-ssm-agent) compiled directly in the Docker build.
* CA certificates and nothing else—no package manager, shell, or extraneous tooling—keeping the runtime attack surface extremely small (`FROM scratch`).
