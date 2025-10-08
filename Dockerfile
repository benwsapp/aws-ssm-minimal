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
RUN apk add --no-cache ca-certificates git patch && \
    git init && \
    git remote add origin https://github.com/aws/amazon-ssm-agent.git && \
    git fetch --depth 1 origin "${SSM_AGENT_REF}" && \
    git checkout FETCH_HEAD

COPY patches/*.patch ./patches/
RUN for patch in patches/*.patch; do \
        echo "Applying ${patch}"; \
        patch -p1 < "${patch}"; \
    done

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/amazon-ssm-agent ./agent

RUN mkdir -p /rootfs/var/lib/amazon/ssm/Vault/Store \
    /rootfs/var/log/amazon/ssm \
    /rootfs/etc/amazon/ssm

RUN cp amazon-ssm-agent.json.template /rootfs/etc/amazon/ssm/amazon-ssm-agent.json && \
    cp seelog_unix.xml /rootfs/etc/amazon/ssm/seelog.xml

RUN chown -R 65533:65533 /rootfs/var /rootfs/etc/amazon

FROM scratch AS runtime

LABEL org.opencontainers.image.source="https://github.com/benwsapp/aws-ssm-minimal" \
      org.opencontainers.image.description="Minimal hardened container image for AWS SSM sessions"

ENV TTL_SECONDS=3600 \
    TTL_SHUTDOWN_GRACE_SECONDS=15

COPY --from=ttl_builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=ttl_builder /out/ttl /ttl
COPY --from=ssm_builder /out/amazon-ssm-agent /service/amazon-ssm-agent
COPY --from=ssm_builder --chown=65533:65533 /rootfs/ /

USER 65533:65533

ENTRYPOINT ["/ttl"]
CMD ["/service/amazon-ssm-agent"]

