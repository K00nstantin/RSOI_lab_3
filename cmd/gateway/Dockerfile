FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gateway ./cmd/gateway/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates wget curl

WORKDIR /root/
COPY --from=builder /app/gateway .
# Копируем wait-for.sh из папки scripts
COPY scripts/wait-for.sh .

EXPOSE 8080
CMD ["./gateway"]