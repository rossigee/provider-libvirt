# Use distroless as minimal base image
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy the pre-built binary from the bin directory
COPY bin/provider .

# Use non-root user from distroless
USER 65532:65532

ENTRYPOINT ["/provider"]