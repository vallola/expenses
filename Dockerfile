FROM golang:1.27-alpine AS builder
RUN apk add --no-cache tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/app/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /server .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/templates ./templates
EXPOSE 8080
CMD ["./server"]
