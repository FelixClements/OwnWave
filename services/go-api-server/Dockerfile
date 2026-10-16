FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:3.22

RUN apk add --no-cache ffmpeg ca-certificates wget \
    && adduser -D -u 1000 ownwave

WORKDIR /app

COPY --from=builder /app/server .
RUN chown ownwave:ownwave /app/server

USER ownwave

EXPOSE 8080

CMD ["./server"]
