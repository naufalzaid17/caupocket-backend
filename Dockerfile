# syntax=docker/dockerfile:1
# ---------------------------------------------------------------------------
# Caupocket API — SATU container berisi backend + Redis (event bus internal)
# + Tesseract (OCR). Trade-off: proses ganda dalam satu container dipilih
# agar deployment self-host sederhana (tanpa service Redis terpisah).
# Untuk skala horizontal, set EVENT_BUS=redis + REDIS_ADDR ke Redis eksternal.
# ---------------------------------------------------------------------------

FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/caupocket-api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache \
      redis \
      tesseract-ocr \
      tesseract-ocr-data-eng \
      ca-certificates \
      tzdata \
 && addgroup -S app && adduser -S app -G app \
 && mkdir -p /data/redis && chown -R app:app /data/redis

COPY --from=builder /out/caupocket-api /usr/local/bin/caupocket-api
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER app
WORKDIR /home/app

# Default untuk mode satu-container: bus Redis lokal.
ENV EVENT_BUS=redis \
    REDIS_ADDR=127.0.0.1:6379 \
    TESSERACT_BIN=/usr/bin/tesseract \
    PORT=8080

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
