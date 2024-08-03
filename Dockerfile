FROM golang:1.22.5 AS builder

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

RUN go build -o main .

FROM debian:bookworm-slim

COPY --from=builder /app/main /app/main
COPY .env /app/.env
COPY static/ /app/static

WORKDIR /app

EXPOSE 9003

CMD ["/app/main"]
