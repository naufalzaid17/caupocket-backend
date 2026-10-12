# Laporan Pengujian Performa & Keamanan — Caupocket v2.0.0

Tanggal: 12–13 Oktober 2026
Pipeline CI: <https://github.com/naufalzaid17/caupocket-backend/actions/runs/38322409438>
Status akhir: **SEMUA HIJAU** (SAST ✓ · E2E ✓ · DAST 18/18 PASS ✓ · Load test ✓ · Frontend build ✓)

> Pengujian dijalankan di GitHub Actions (ubuntu-latest, PostgreSQL 16 service
> container) karena Smart App Control pada mesin development memblokir eksekusi
> binary hasil kompilasi lokal.

---

## 1. SAST (Static Application Security Testing)

| Alat | Lingkup | Hasil |
|---|---|---|
| `gosec` v2 | 41 file Go (CWE-78, 190, 400, 703, 798…) | **0 temuan aktif** (6 awal: 1 diperbaiki nyata — G112 Slowloris; 2 di-handle eksplisit; 1 false-positive diberi `#nosec` beralasan; 2 G104 `_ =` eksplisit) |
| `govulncheck` | seluruh call graph | **0 kerentanan pada kode yang ter-call** (upgrade `x/net` 0.58→0.60, toolchain go1.27.0→1.27.2, `quic-go` 0.59.0→0.59.1) |
| `npm audit` (frontend) | dependency web | **0 vulnerabilities** (upgrade astro 5.18.2→7.3.8 + @astrojs/node 11.1.7 menutup advisori **critical** RCE-AVIF/XSS-define:vars/SSRF dan **high** Host-header-crash) |
| Scan rahasia ter-commit | `git grep` pola kredensial + file `.env/.pem/.key` | **bersih** (`.env` ter-gitignore, JWT secret hanya di env) |

## 2. DAST & Simulasi Pentest (18 probe, semuanya PASS)

Dijalankan `scripts/dast-probes.mjs` terhadap API + web nyama di CI.

| Probe | Hasil |
|---|---|
| A01 — 6 endpoint terproteksi tanpa token | ✅ 401 semua |
| A02a JWT `alg=none` | ✅ 401 |
| A02b JWT tanda tangan salah | ✅ 401 |
| A02c JWT kadaluarsa | ✅ 401 |
| A02d JWT `sub` bukan UUID | ✅ 401 |
| A03a SQLi login (3 payload klasik) | ✅ 400/401 parameterized, tanpa error DB |
| A03b SQLi path parameter goal | ✅ 400 |
| A04 IDOR — user B mendanai goal user A | ✅ 404, saldo tak tersentuh |
| A07 XSS tersimpan → render dashboard | ✅ ter-escape (`&lt;script&gt;`) |
| A05 CSRF POST cross-origin (evil.example) | ✅ 403 checkOrigin Astro |
| Cookie sesi | ✅ `HttpOnly; Secure; SameSite=Lax` |
| Nominal negatif/nol | ✅ 400 |
| Upload OCR > 5MB | ✅ 413 |
| Upload non-gambar (magic bytes) | ✅ 400 |
| Enumerasi user (email vs password salah) | ✅ pesan identik |
| Security header API | ✅ nosniff / DENY / strict-origin-when-cross-origin |
| Security header web | ✅ nosniff / DENY (middleware Astro) |
| Rate limit brute-force login | ✅ 429 + Retry-After setelah kuota |

### Temuan → perbaikan (pra vs pasca)

| Temuan awal (bukti pra-perbaikan) | Perbaikan | Verifikasi |
|---|---|---|
| Tidak ada security header sama sekali | `middleware.SecurityHeaders()` (backend) + `src/middleware.ts` (web); HSTS otomatis saat TLS | probe header PASS |
| 12 login salah beruntun diproses semua (401×12, tanpa hambatan) | Rate limit per-IP fixed-window (login 12/menit, register 5/menit, OCR 10/menit; env `RATE_LIMIT_*`, 0=off) + header `Retry-After` | 429 terpicu ✅ |
| Swagger terekspos di mode release | Toggle `SWAGGER_ENABLED` (default true untuk dev) | terdokumentasi |
| `ClientIP` Gin bisa dipalsukan via `X-Forwarded-For` (bypass rate limiter) | `SetTrustedProxies(nil)` | by-design aman |
| G112 Slowloris (`ReadHeaderTimeout` kosong) | `ReadHeaderTimeout 10s`, Read/Write/Idle timeout | gosec bersih |
| Advisori critical pada astro ≤7.2.7 | Upgrade astro 7.3.8 + node adapter 11.1.7 | npm audit 0 |

## 3. Performance / Load Test

Runner GitHub Actions 2-vCPU (ubuntu-latest), PostgreSQL 16 container,
`scripts/loadtest` (Go, keep-alive). Rate limit login dinonaktifkan hanya pada
sesi pengukuran agar throughput bcrypt terlihat murni.

| Skenario | Konkurensi | Throughput | p50 | p95 | p99 | Error |
|---|---|---|---|---|---|---|
| read (me/wallets/transactions/goals) | 25 · 15s | **1.417 req/s** | 3,8 ms | 111 ms | 160 ms | 0% |
| write (POST transactions, DB TX + event) | 10 · 10s | **791 req/s** | 3,7 ms | 34 ms | 234 ms | 0% |
| login (bcrypt cost 12) | 10 · 10s | **13,5 req/s** | 724 ms | 835 ms | 899 ms | 0% |

Interpretasi:
- Endpoint baca nyaman untuk ribuan pengguna aktif pada 1 instance kecil.
- Write path tetap >700 RPS meski setiap insert membuka DB transaction penuh
  (row lock + publish event) — ACID tidak mengorbankan throughput.
- Login memang CPU-bound karena bcrypt cost 12 — **by design** (brute-force
  mahal secara komputasi), dilindungi tambahan rate limit 12/menit/IP.

## 4. E2E Fungsional (regresi)

`go test -tags e2e` — ledger ACID penuh + habit engine: cash-out validasi
saldo, rollback penuh saat funding gagal, goal auto-ACHIEVED, reward streak
tepat satu kali, toggle off tanpa reward. **LOLOS**.

## 5. Risiko residual / catatan opsional

- Rate limiter & event bus `memory` bersifat per-instance; untuk multi-replica
  gunakan `EVENT_BUS=redis` + limiter ber-Redis.
- Swagger UI: set `SWAGGER_ENABLED=false` di production.
- Cookie `Secure` membutuhkan HTTPS di belakang reverse proxy.
- Endpoint register 409 mengungkap keberadaan email (trade-off UX umum);
  pertimbangkan verifikasi email bila diperlukan.

## 6. Reproduksi

```bash
# Semua via CI (butuh repo + GitHub):
gh workflow run security-perf.yml --ref main

# DAST lokal (API + web berjalan):
DAST_JWT_SECRET=<rahasia> node scripts/dast-probes.mjs

# Load test lokal:
go run ./scripts/loadtest -base http://localhost:8080/api/v1 -scenario read -c 25 -dur 15s
```
