FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY . .
RUN go mod tidy
RUN go build -o server .

FROM alpine:latest

RUN apk add --no-cache ffmpeg ca-certificates wget \
    && adduser -D -u 1000 ownwave

WORKDIR /app

COPY --from=builder /app/server .
RUN chown ownwave:ownwave /app/server

USER ownwave

EXPOSE 8080

CMD ["./server"]
