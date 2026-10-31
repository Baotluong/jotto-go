# syntax=docker/dockerfile:1

# 1. Build the React frontend.
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. Build the Go server with the frontend embedded. Pure-Go SQLite means no cgo.
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /jotto ./cmd/server

# 3. Ship a single static binary.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /jotto /jotto
ENV PORT=8080 DB_PATH=/data/jotto.db
EXPOSE 8080
ENTRYPOINT ["/jotto"]
