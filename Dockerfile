FROM alpine:latest

# Create workdir
RUN set -e \
    && mkdir /app
WORKDIR /app

# Install dependencies
RUN set -e \
    && apk add --no-cache postgresql-client postgresql ca-certificates

# Update certificates
RUN update-ca-certificates

# Copy application
COPY bin/test-environment-manager /app
COPY public /app/public/

ENTRYPOINT ["/app/test-environment-manager"]
