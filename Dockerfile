# Stage 1: Build the binary
FROM golang:1.24-alpine AS builder

# Build එකට අවශ්‍ය tools install කිරීම
RUN apk add --no-cache git

WORKDIR /app

# Dependencies install කිරීම
COPY go.mod go.sum ./
RUN go mod download

# Source code එක copy කරලා build කිරීම
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Final lightweight image
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Builder stage එකෙන් binary එක විතරක් ගමු
COPY --from=builder /app/main .
# ඔයාට .env file එකක් තියෙනවා නම් ඒකත් copy කරන්න
COPY --from=builder /app/.env . 

# App එක දුවන Port එක (සාමාන්‍යයෙන් 8080)
EXPOSE 8090

CMD ["./main"]