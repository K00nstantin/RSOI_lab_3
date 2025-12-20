FROM golang:1.23-alpine AS builder

WORKDIR /app

# Сначала копируем только go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Затем копируем ВСЕ исходные файлы
COPY . .

# Собираем приложение
RUN go build -o rating ./cmd/rating/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates wget curl

WORKDIR /root/
COPY --from=builder /app/rating .
COPY scripts/wait-for.sh .

EXPOSE 8050
CMD ["./rating"]