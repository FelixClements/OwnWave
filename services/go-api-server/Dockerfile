# Build context is the repository root (it needs both the API and the web UI).
FROM node:20-alpine AS web

WORKDIR /app

COPY apps/t3-web-frontend/package.json apps/t3-web-frontend/package-lock.json ./
RUN npm ci

COPY apps/t3-web-frontend ./
RUN npm run build

FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY services/go-api-server/go.mod services/go-api-server/go.sum ./
RUN go mod download

COPY services/go-api-server ./
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:3.22

RUN apk add --no-cache ffmpeg ca-certificates wget \
    && adduser -D -u 1000 ownwave

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=web /app/out ./web
COPY database/migrations /migrations
RUN chown ownwave:ownwave /app/server

USER ownwave

EXPOSE 8080

CMD ["./server"]
