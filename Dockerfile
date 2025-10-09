ARG GO_VERSION=1.25.1
ARG SSM_AGENT_REF=mainline

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS ttl_builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/ttl ./cmd/ttl

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS ssm_builder

ARG TARGETOS
ARG TARGETARCH
ARG SSM_AGENT_REF

WORKDIR /src

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates git patch busybox-static && \
    git init && \
    git remote add origin https://github.com/aws/amazon-ssm-agent.git && \
    FETCH_REF="${SSM_AGENT_REF}" && \
    if ! git fetch --depth 1 origin "${FETCH_REF}"; then \
        case "${FETCH_REF}" in \
            refs/tags/v*) \
                ALT_REF="refs/tags/${FETCH_REF#refs/tags/v}" && \
                git fetch --depth 1 origin "${ALT_REF}" && \
                FETCH_REF="${ALT_REF}" ;; \
            refs/tags/*) \
                ALT_REF="refs/tags/v${FETCH_REF#refs/tags/}" && \
                git fetch --depth 1 origin "${ALT_REF}" && \
                FETCH_REF="${ALT_REF}" ;; \
            *) \
                exit 1 ;; \
        esac; \
    fi && \
    git checkout FETCH_HEAD

COPY patches/*.patch ./patches/
SHELL ["/bin/ash", "-o", "pipefail", "-c"]
RUN for patch in patches/*.patch; do \
        echo "Applying ${patch}"; \
        patch -p1 < "${patch}"; \
    done
SHELL ["/bin/sh", "-c"]

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

SHELL ["/bin/ash", "-o", "pipefail", "-c"]
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    VERSION=$(printf '%s' "${SSM_AGENT_REF##*/}" | sed 's/^v//') && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w -X github.com/aws/amazon-ssm-agent/agent/version.Version=${VERSION}" -o /out/amazon-ssm-agent ./agent && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/ssm-document-worker ./agent/framework/processor/executer/outofproc/worker && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/ssm-session-worker ./agent/framework/processor/executer/outofproc/sessionworker && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/ssm-session-logger ./agent/session/logging
SHELL ["/bin/sh", "-c"]

RUN mkdir -p /rootfs/var/lib/amazon/ssm/Vault/Store \
    /rootfs/var/lib/amazon/ssm/runtimeconfig \
    /rootfs/var/lib/amazon/ssm/dynamicconfig \
    /rootfs/var/lib/amazon/ssm/ipc \
    /rootfs/var/log/amazon/ssm \
    /rootfs/etc/amazon/ssm \
    /rootfs/bin

RUN cp /bin/busybox /rootfs/bin/busybox && \
    ln -sf busybox /rootfs/bin/sh

RUN touch /rootfs/var/lib/amazon/ssm/runtimeconfig/identity_config.json && \
    touch /rootfs/var/log/amazon/ssm/amazon-ssm-agent.log && \
    touch /rootfs/var/log/amazon/ssm/errors.log && \
    chown 65533:65533 /rootfs/var/lib/amazon/ssm/runtimeconfig/identity_config.json /rootfs/var/log/amazon/ssm/amazon-ssm-agent.log /rootfs/var/log/amazon/ssm/errors.log && \
    chmod 0664 /rootfs/var/lib/amazon/ssm/runtimeconfig/identity_config.json /rootfs/var/log/amazon/ssm/amazon-ssm-agent.log /rootfs/var/log/amazon/ssm/errors.log

COPY assets/amazon-ssm-agent.json /rootfs/etc/amazon/ssm/amazon-ssm-agent.json
COPY assets/seelog.xml /rootfs/etc/amazon/ssm/seelog.xml
COPY assets/sessionlogger/seelog.xml /rootfs/etc/amazon/ssm/sessionlogger/seelog.xml

RUN chown -R 65533:65533 /rootfs/var /rootfs/etc/amazon && \
    chmod -R 0775 /rootfs/var/lib/amazon/ssm /rootfs/var/log/amazon/ssm && \
    chmod 0775 /rootfs/var/lib/amazon/ssm/runtimeconfig

FROM scratch AS runtime

LABEL org.opencontainers.image.source="https://github.com/benwsapp/aws-ssm-minimal" \
      org.opencontainers.image.description="Minimal hardened container image for AWS SSM sessions"

ENV TTL_SECONDS=3600 \
    TTL_SHUTDOWN_GRACE_SECONDS=15

COPY --from=ttl_builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=ttl_builder /out/ttl /ttl
COPY --from=ssm_builder /out/amazon-ssm-agent /service/amazon-ssm-agent
COPY --from=ssm_builder /out/ssm-document-worker /service/ssm-document-worker
COPY --from=ssm_builder /out/ssm-session-worker /service/ssm-session-worker
COPY --from=ssm_builder /out/ssm-session-logger /service/ssm-session-logger
COPY --from=ssm_builder /out/amazon-ssm-agent /usr/bin/amazon-ssm-agent
COPY --from=ssm_builder /out/ssm-document-worker /usr/bin/ssm-document-worker
COPY --from=ssm_builder /out/ssm-session-worker /usr/bin/ssm-session-worker
COPY --from=ssm_builder /out/ssm-session-logger /usr/bin/ssm-session-logger
COPY --from=ssm_builder --chown=65533:65533 /rootfs/ /

USER 65533:65533

ENTRYPOINT ["/ttl"]
CMD ["/service/amazon-ssm-agent"]

