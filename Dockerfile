# syntax=docker.io/docker/dockerfile:1

# ========= BUILDER =========
FROM golang:1.26-alpine AS builder
WORKDIR /app

# gcc & musl-dev are required for CGO (glebarez/sqlite uses CGO)
RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o server .

# ========= RUNNER =========
FROM alpine:3.23
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Set timezone to Jakarta (UTC+7)
ENV TZ=Asia/Jakarta

# Copy binary
COPY --from=builder /app/server /app/server

# Copy config
COPY --from=builder /app/config.yaml /app/config.yaml

# Copy view templates (Fiber html/template engine)
COPY --from=builder /app/templates /app/templates

# Copy i18n locale files if present
COPY --from=builder /app/localize /app/localize

# Copy public assets
COPY --from=builder /app/public /app/public

# Create runtime directories
RUN mkdir -p /app/logs /app/uploads

USER 0:0

EXPOSE 2005

CMD ["./server"]