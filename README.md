# Caupocket API

RESTful API untuk Caupocket — personal finance & goal-based saving planner.
Dibangun dengan Golang (Gin + GORM) mengikuti Clean Architecture, dengan
jaminan integritas ACID pada setiap mutasi dana.

## Tech Stack

| Komponen | Pilihan |
|---|---|
| HTTP Framework | Gin |
| Database | PostgreSQL (via GORM) |
| Autentikasi | JWT (HS256, `golang-jwt/jwt/v5`) |
| Password hashing | bcrypt (`golang.org/x/crypto`) |
| Dokumentasi | Swagger (`swaggo/swag`) |
| Config | `.env` (`godotenv`) |

## Struktur (Clean Architecture)

```text
cmd/api/            Entry point + graceful shutdown
internal/app/       Wiring seluruh layer + router
internal/config/    Parser .env (DATABASE_URL, JWT_SECRET, dst.)
internal/model/     Struct domain murni (UUID, BIGINT amount, soft delete)
internal/repository/ Query PostgreSQL (TX-aware: FindForUpdate, ApplyBalanceDelta)
internal/service/   Business logic (validasi saldo, goal funding, TX boundary)
internal/handler/   HTTP handler + DTO + anotasi Swagger
internal/middleware/ JWT AuthRequired, CORS
pkg/                response (Standard Response Wrapper), password (bcrypt)
docs/               Hasil generate swag init
e2e/                Test end-to-end (embedded PostgreSQL)
```

## Menjalankan

```bash
# Cara termudah (PostgreSQL + API&Redis dalam satu container):
docker compose up --build

# Atau manual:
docker compose up -d postgres       # atau: go run ./scripts/devpg
cp .env.example .env
go run ./cmd/api
```

Skema database dibuat otomatis via `AutoMigrate` saat startup.
Swagger UI: <http://localhost:8080/swagger/index.html>

### Container "all-in-one" (API + Redis + Tesseract)

Image [`Dockerfile`](Dockerfile) memaketkan binary API, **Redis** (event bus
internal, `--save ''` in-memory, hanya `127.0.0.1` di dalam container), dan
**Tesseract OCR** beserta data bahasa `eng`. Entrypoint
([`docker/entrypoint.sh`](docker/entrypoint.sh)) menyalakan Redis, menunggu
`PING`, lalu menjalankan API — dan meneruskan `SIGTERM` ke keduanya agar
graceful shutdown tetap bekerja.

- Default image: `EVENT_BUS=redis`, `REDIS_ADDR=127.0.0.1:6379` — habit engine
  langsung jalan tanpa service tambahan.
- Ingin Redis terpisah / klaster? Override env `REDIS_ADDR` ke host Redis
  eksternal (atau `EVENT_BUS=memory` untuk single-instance tanpa Redis).
- Catatan arsitektur: dua proses dalam satu container dipilih demi kesederhanaan
  self-hosting; untuk horizontal scaling, pisahkan Redis kembali.

## Testing

```bash
go test ./pkg/...              # unit (bcrypt)
go test -tags e2e ./e2e        # end-to-end dengan embedded PostgreSQL
```

Test E2E tidak membutuhkan Docker — binary PostgreSQL diunduh otomatis ke
direktori temp dan dijalankan pada port 5433.

## Endpoint Ringkas

| Method | Path | Auth | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/auth/register` | - | Daftar (+ dompet & kategori default) |
| POST | `/api/v1/auth/login` | - | Login → JWT |
| GET | `/api/v1/me` | JWT | Profil user token |
| GET | `/api/v1/wallets` | JWT | Daftar dompet + saldo |
| GET/POST | `/api/v1/categories` | JWT | Kategori |
| GET/POST | `/api/v1/transactions` | JWT | Ledger (INCOME/EXPENSE) |
| GET/POST | `/api/v1/goals` | JWT | Target tabungan |
| POST | `/api/v1/goals/{id}/fund` | JWT | Setor dana ke goal (GOAL_FUNDING) |
| POST | `/api/v1/ocr/receipt` | JWT | Pindai struk (OCR Tesseract) → merchant, total, tanggal |
| GET/PUT | `/api/v1/habits/settings` | JWT | Status & toggle auto-nabung |
| GET | `/api/v1/habits/logs` | JWT | Riwayat reward habit engine |

## Habit Engine — Event-Driven Auto-Funding (EPIC V2.2)

Alur event-driven untuk reward kebiasaan hemat:

1. `CreateTransaction` sukses → publish event `transaction.created`
   **setelah TX commit** (async, best-effort; kegagalan publish tidak
   membatalkan transaksi).
2. `internal/worker` me-listen event melalui **EventBus**:
   * `memory` (default) — dispatcher antar-goroutine, sekuensial, tanpa
     infrastruktur eksternal;
   * `redis` — Redis Pub/Sub (`EVENT_BUS=redis` + `REDIS_ADDR`) untuk
     deployment multi-instance.
3. `HabitService` menjalankan aturan: bila user mengaktifkan auto-nabung dan
   sudah **N hari tanpa pengeluaran** (default 3, dihitung dari tanggal
   EXPENSE terakhir / registrasi), sistem otomatis menjalankan `FundGoal`
   (TX ACID yang sama dengan setoran manual) sebesar reward (default
   Rp10.000, env `HABIT_STREAK_DAYS`/`HABIT_REWARD_AMOUNT`) ke goal aktif
   tertua, lalu mencatatnya di tabel `habit_logs`. Satu streak hanya
   direward satu kali (guard berbasis tanggal).

Tabel baru via AutoMigrate: `habit_settings` (toggle per user),
`habit_logs` (riwayat reward).

## Receipt OCR (EPIC V2.1)

`POST /api/v1/ocr/receipt` menerima multipart/form-data (`file`, wajib
`.jpg/.jpeg/.png/.webp` ≤5MB, isi file diverifikasi via magic bytes) dan:

1. Menjalankan binary **Tesseract** sebagai subprocess (`--psm 4`, tanpa cgo):
   `tesseract <img> stdout -l eng --psm 4`.
2. Mem-parsing raw text dengan regex: nama merchant (baris teratas yang
   didominasi huruf), total nominal (baris TOTAL/jumlah/bayar — subtotal
   dikecualikan; fallback baris ber-Rp; separator titik/koma dinormalisasi),
   dan tanggal (dinormalisasi ke YYYY-MM-DD).
3. Mengembalikan DTO `{ merchant_name, total_amount, date, raw_text }`.

Konfigurasi env: `TESSERACT_BIN` (default `tesseract`; di Windows installer
UB Mannheim biasanya `C:\Program Files\Tesseract-OCR\tesseract.exe`),
`TESSERACT_LANG` (default `eng`), `OCR_TIMEOUT` (default `30s`).

Uji parser tanpa Tesseract: `go test ./internal/service/...`.
Buat gambar struk dummy: `powershell -File scripts/make-test-receipt.ps1`.

## Jaminan Transaksional

`Cash-Out` dan `Goal Funding` masing-masing berjalan di dalam **satu**
database transaction:

1. Kunci baris wallet (`SELECT ... FOR UPDATE`) + validasi kepemilikan & saldo.
2. Mutasi `wallets.balance`.
3. (Goal funding) tambah `saving_goals.current_amount`, status otomatis
   `ACHIEVED` saat target tercapai.
4. Insert ke `transactions`.

Gagal di langkah mana pun → rollback penuh, tidak ada saldo korup.
