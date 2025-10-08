> [!WARNING]
> Project is still in progress. This project is designed to be coupled with another project (not yet public) that will launch an SSM session to access private EKS clusters.

# aws-ssm-minimal

[![Go](https://img.shields.io/badge/go-1.25-00ADD8.svg?logo=go)](https://tip.golang.org/doc/go1.25)
[![Go Report Card](https://goreportcard.com/badge/github.com/benwsapp/aws-ssm-minimal)](https://goreportcard.com/report/github.com/benwsapp/aws-ssm-minimal)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.md)
[![Test Status](https://github.com/benwsapp/aws-ssm-minimal/actions/workflows/ci.yml/badge.svg)](https://github.com/benwsapp/aws-ssm-minimal/actions/workflows/ci.yml)

`aws-ssm-minimal` is a purpose-built container image for running the AWS Systems Manager (SSM) agent as a sidecar in compute environments that do not ship with SSM pre-installed (for example, ECS Fargate tasks, EKS Pods, or plain OCI runtimes). The pattern matches AWS’s own guidance for ECS Fargate based SSM sessions, and the image bundles:

* A lightweight TTL wrapper written in Go that supervises the agent, adds a configurable time-to-live, forwards signals, and performs activation clean-up when the container exits.
* A non-root build of the official [`aws/amazon-ssm-agent`](https://github.com/aws/amazon-ssm-agent) compiled directly in the Docker build.
* CA certificates (to interact with AWS APIs) and binaries only - the runtime attack surface extremely small (`FROM scratch`).
