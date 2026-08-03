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

COPY --chmod=0755 bin/${TARGETOS}_${TARGETARCH}/provider /usr/local/bin/provider

ENTRYPOINT ["/usr/local/bin/provider"]