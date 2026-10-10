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
# 1. Siapkan PostgreSQL (opsional: docker-compose bawaan)
docker compose up -d

# 2. Salin env lalu sesuaikan
cp .env.example .env

# 3. Jalankan server (port default 8080)
go run ./cmd/api
```

Skema database dibuat otomatis via `AutoMigrate` saat startup.
Swagger UI: <http://localhost:8080/swagger/index.html>

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

## Jaminan Transaksional

`Cash-Out` dan `Goal Funding` masing-masing berjalan di dalam **satu**
database transaction:

1. Kunci baris wallet (`SELECT ... FOR UPDATE`) + validasi kepemilikan & saldo.
2. Mutasi `wallets.balance`.
3. (Goal funding) tambah `saving_goals.current_amount`, status otomatis
   `ACHIEVED` saat target tercapai.
4. Insert ke `transactions`.

Gagal di langkah mana pun → rollback penuh, tidak ada saldo korup.
