FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy the source code
COPY backend/ ./

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -o profhit-server .

# Start a new stage from scratch
FROM alpine:latest  
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/profhit-server .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./profhit-server"]
