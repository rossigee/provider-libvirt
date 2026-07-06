FROM alpine:3.19

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache libvirt libvirt-client util-linux

COPY bin/provider-${TARGETOS}-${TARGETARCH} /usr/local/bin/provider

ENTRYPOINT ["provider"]
