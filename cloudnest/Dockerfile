FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o cloudnest .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/cloudnest .
RUN mkdir -p /app/data
ENV GIN_MODE=release
ENV CLOUDNEST_DB_TYPE=sqlite
ENV CLOUDNEST_DB_DSN=/app/data/cloudnest.db
ENV CLOUDNEST_LISTEN=0.0.0.0:8800
EXPOSE 8800
CMD ["/app/cloudnest", "server"]
