# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /web
COPY cloudnest-web/package*.json ./
RUN npm ci
COPY cloudnest-web/ ./
RUN npm run build

# Stage 2: Cross-compile agent binaries
FROM golang:1.24-alpine AS agent-builder
WORKDIR /build
COPY cloudnest-agent/go.mod cloudnest-agent/go.sum ./
RUN go mod download
COPY cloudnest-agent/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cloudnest-agent-linux-amd64 . && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o cloudnest-agent-linux-arm64 .

# Stage 3: Build master (with embedded frontend)
FROM golang:1.24-alpine AS master-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY cloudnest/go.mod cloudnest/go.sum ./
RUN go mod download
COPY cloudnest/ ./
# Copy frontend build into embed directory
COPY --from=frontend /web/dist/ ./public/dist/
RUN CGO_ENABLED=1 go build -o cloudnest .

# Stage 4: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Copy master binary
COPY --from=master-builder /build/cloudnest .

# Copy agent binaries for download
RUN mkdir -p /app/data/binaries
COPY --from=agent-builder /build/cloudnest-agent-linux-amd64 /app/data/binaries/
COPY --from=agent-builder /build/cloudnest-agent-linux-arm64 /app/data/binaries/

ENV GIN_MODE=release
ENV CLOUDNEST_LISTEN=0.0.0.0:8800
ENV CLOUDNEST_DB_TYPE=mysql
ENV CLOUDNEST_DB_DSN=cloudnest:change-me@tcp(save_vps_db:3306)/cloudnest?charset=utf8mb4&parseTime=True&loc=Local
ENV CLOUDNEST_REG_TOKEN=change-me-reg-token
ENV CLOUDNEST_SIGNING_SECRET=change-me-signing-secret

EXPOSE 8800
VOLUME /app/data

CMD ["/app/cloudnest", "server"]
