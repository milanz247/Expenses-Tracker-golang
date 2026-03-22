# Stage 1: Build the binary
FROM golang:1.24-alpine AS builder

# Install necessary tools for the build
RUN apk add --no-cache git

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Final lightweight image
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Only copy the binary from the builder stage
COPY --from=builder /app/main .
# The port the app runs on (usually 8080)
EXPOSE 8081

CMD ["./main"]