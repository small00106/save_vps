FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o cloudnest-agent .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/cloudnest-agent .
RUN mkdir -p /data
ENV GIN_MODE=release
EXPOSE 8801
CMD ["/app/cloudnest-agent", "run"]
