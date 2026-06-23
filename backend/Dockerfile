FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /profhit-backend .

FROM alpine:latest

WORKDIR /

COPY --from=builder /profhit-backend /profhit-backend

EXPOSE 8080

CMD ["/profhit-backend"]
