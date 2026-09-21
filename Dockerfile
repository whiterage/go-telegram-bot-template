# ---- build stage ----
FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bot ./cmd/bot

# ---- runtime stage ----
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bot .

# Каталог для SQLite-базы (монтируется как volume)
RUN mkdir -p /app/data
ENV DB_PATH=/app/data/data.db

EXPOSE 8080
CMD ["./bot"]
