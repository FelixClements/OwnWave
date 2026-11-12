# Build context is the repository root (it needs both the API and the web UI).
FROM node:20-alpine AS web

WORKDIR /app

COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci

COPY apps/web ./
RUN npm run build

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY services/go-api-server/go.mod services/go-api-server/go.sum ./
RUN go mod download

COPY services/go-api-server ./
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:3.22

RUN apk add --no-cache ffmpeg ca-certificates wget \
    && adduser -D -u 1000 ownwave \
    && mkdir -p /cache/transcodes && chown -R ownwave:ownwave /cache

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=web /app/dist ./web
COPY database/migrations /migrations
RUN chown ownwave:ownwave /app/server

USER ownwave

EXPOSE 8080

CMD ["./server"]
