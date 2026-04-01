# Build stage
FROM golang:1.23-alpine AS builder

# Build arguments for version info
ARG VERSION
ARG GITCOMMIT

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev linux-headers

# Set working directory to match GOPATH structure
WORKDIR /go/src/github.com/beatoz/beatoz-go

# Copy all source code
COPY . .

# Download dependencies
RUN go mod download

# Build the binary with version info
RUN VERTAG=${VERSION} GITCOMMIT=${GITCOMMIT} make linux

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary and entrypoint script from builder
COPY --from=builder /go/src/github.com/beatoz/beatoz-go/build/linux/beatoz /usr/local/bin/beatoz
COPY docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# Set working directory
WORKDIR /root

# Make entrypoint script executable
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Initialize beatoz with predefined settings
ENV BEATOZ_VALIDATOR_SECRET="unsafe_password" \
    BEATOZ_HOLDER_SECRET="unsafe_password" \
    BEATOZ_WALKEY_SECRET="unsafe_password"

# Expose ports (adjust as needed)
EXPOSE 26656 26657 26658 26660

# Use entrypoint script
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["start", "--home", "/root/.beatoz", "--rpc.laddr", "tcp://0.0.0.0:26657", "--rpc.cors_allowed_origins", "*"]