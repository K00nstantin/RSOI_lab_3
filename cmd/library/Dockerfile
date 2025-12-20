FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o library ./cmd/library/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates wget curl

WORKDIR /root/
COPY --from=builder /app/library .
COPY scripts/wait-for.sh .

EXPOSE 8060
CMD ["./library"]