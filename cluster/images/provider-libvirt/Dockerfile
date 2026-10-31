# Stage 1: Build the binary inside ubuntu:26.04 with matching libvirt-dev
FROM ubuntu:26.04 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
    golang \
    gcc \
    libc6-dev \
    pkg-config \
    libvirt-dev \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy source code and dependencies
COPY cmd ./cmd
COPY internal ./internal
COPY apis ./apis
COPY go.mod go.sum ./
COPY vendor ./vendor

# Build the binary inside the container matching target libvirt version
RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -mod=vendor -o bin/${TARGETOS}_${TARGETARCH}/provider \
    -ldflags '-X github.com/rossigee/provider-libvirt/internal/version.Version=${VERSION}' \
    ./cmd/provider

# Stage 2: Runtime image with only the binary and runtime deps
FROM ubuntu:26.04

ARG TARGETOS
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
    libvirt-clients \
    libvirt0 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

LABEL org.opencontainers.image.title="provider-libvirt"
LABEL org.opencontainers.image.description="Crossplane provider for libvirt/KVM management"
LABEL org.opencontainers.image.vendor="rossigee"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/rossigee/provider-libvirt"
LABEL org.opencontainers.image.url="https://github.com/rossigee/provider-libvirt"
LABEL org.opencontainers.image.documentation="https://github.com/rossigee/provider-libvirt/blob/master/README.md"

COPY --chmod=0755 --from=builder /src/bin/${TARGETOS}_${TARGETARCH}/provider /usr/local/bin/provider

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/provider"]
