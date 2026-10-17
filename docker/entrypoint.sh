#!/bin/sh
# Entrypoint satu-container: Redis (bus internal) + Caupocket API.
# Shell tetap jadi PID 1 agar SIGTERM diteruskan ke kedua proses
# (API graceful shutdown; Redis in-memory tanpa persistensi).
set -e

echo "[entrypoint] menjalankan redis-server (bus internal)..."
redis-server \
  --port 6379 \
  --bind 127.0.0.1 \
  --dir /data/redis \
  --save '' \
  --appendonly no \
  --daemonize no &
REDIS_PID=$!

for i in $(seq 1 50); do
  if redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q PONG; then
    break
  fi
  sleep 0.2
done
if ! redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q PONG; then
  echo "[entrypoint] redis gagal berjalan" >&2
  exit 1
fi
echo "[entrypoint] redis siap (127.0.0.1:6379) — menjalankan API..."

/usr/local/bin/caupocket-api &
API_PID=$!

shutdown() {
  echo "[entrypoint] mematikan proses..."
  kill "$API_PID" "$REDIS_PID" 2>/dev/null || true
  wait "$API_PID" "$REDIS_PID" 2>/dev/null || true
  exit 0
}
trap shutdown TERM INT

wait "$API_PID"
RC=$?
kill "$REDIS_PID" 2>/dev/null || true
exit "$RC"
