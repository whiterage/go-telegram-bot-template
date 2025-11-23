FROM golang:1.25-alpine AS builder
EXPOSE 8080
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/tgbot_unihack
COPY --from=builder /app/bot .
# Копируем .env файл в образ
COPY .env .
# Создаем директорию для данных
RUN mkdir -p /root/tgbot_unihack
VOLUME ["/root/tgbot_unihack"]
CMD ["./bot"]