# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Download dependencies first so this layer is cached separately from source.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and compile.
COPY . .

# SERVICE is injected at build time to select which cmd/ entry-point to build.
# Valid values: orchestrator | api | metering
ARG SERVICE=orchestrator

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /bin/service \
    ./cmd/${SERVICE}/

# Runtime stage — minimal image with only CA certs.
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Run as non-root.
RUN addgroup -S proxycoin && adduser -S proxycoin -G proxycoin
USER proxycoin

COPY --from=builder /bin/service /usr/local/bin/service

ENTRYPOINT ["/usr/local/bin/service"]
