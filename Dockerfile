FROM golang:1.26.8-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /profhit-server .
FROM alpine:3.23
RUN apk --no-cache add ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /profhit-server /app/profhit-server
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
CMD ["/app/profhit-server"]
