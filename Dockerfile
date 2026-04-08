# Stage 1: Build frontend
ARG BUILDPLATFORM
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
RUN apk add --no-cache bash
WORKDIR /src
COPY scripts/ ./scripts/
COPY cloudnest-web/ ./cloudnest-web/
RUN chmod +x ./scripts/build-assets.sh && ./scripts/build-assets.sh frontend --output /out/dist

# Stage 2: Cross-compile agent binaries
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS agent-builder
RUN apk add --no-cache bash
WORKDIR /src
COPY scripts/ ./scripts/
COPY cloudnest-agent/ ./cloudnest-agent/
RUN chmod +x ./scripts/build-assets.sh && ./scripts/build-assets.sh agent --output /out

# Stage 3: Build master (with embedded frontend)
FROM golang:1.24-alpine AS master-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY cloudnest/go.mod cloudnest/go.sum ./
RUN go mod download
COPY cloudnest/ ./
# Copy frontend build into embed directory
COPY --from=frontend /out/dist/ ./public/dist/
RUN CGO_ENABLED=1 go build -o cloudnest .

# Stage 4: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Copy master binary
COPY --from=master-builder /build/cloudnest .

# Copy agent binaries for download
RUN mkdir -p /app/data/binaries
COPY --from=agent-builder /out/cloudnest-agent-linux-amd64 /app/data/binaries/
COPY --from=agent-builder /out/cloudnest-agent-linux-arm64 /app/data/binaries/

ENV GIN_MODE=release
ENV CLOUDNEST_LISTEN=0.0.0.0:8800
ENV CLOUDNEST_DB_TYPE=sqlite
ENV CLOUDNEST_DB_DSN=/app/data/cloudnest.db

EXPOSE 8800
VOLUME /app/data
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=5 CMD wget -q -O - http://127.0.0.1:8800/healthz >/dev/null 2>&1 || exit 1

CMD ["/app/cloudnest", "server"]
