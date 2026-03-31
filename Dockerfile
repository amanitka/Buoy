# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o buoy .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/buoy .

# Create non-root user
RUN addgroup -g 1000 buoy && \
    adduser -D -u 1000 -G buoy buoy && \
    chown -R buoy:buoy /root

USER buoy

EXPOSE 8080

CMD ["./buoy"]
