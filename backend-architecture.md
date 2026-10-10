# Caupocket - Backend Architecture Document

## 1. System Overview
Caupocket adalah sistem pencatatan keuangan pribadi (Personal Finance & Goal-Based Saving Planner). Backend ini dirancang sebagai *RESTful API Service* yang bersifat *stateless*, mengedepankan integritas data transaksional (ACID compliance), dan menggunakan pendekatan struktur *Clean Architecture*.

* **Tech Stack:** Golang (Gin / Fiber)
* **Database:** PostgreSQL
* **Documentation:** Swagger (OpenAPI via `swaggo/swag`)
* **Authentication:** JWT (JSON Web Token)

---

## 2. Architectural Pattern: Clean Architecture
Sistem memisahkan *concern* ke dalam 4 layer utama berdasarkan prinsip *Dependency Rule* (komunikasi hanya boleh dari luar ke dalam).

1. **Model/Domain Layer:** Struktur entitas murni Golang (representasi tabel *database*). Tidak bergantung pada *library* eksternal.
2. **Repository Layer:** Satu-satunya layer yang berinteraksi dengan PostgreSQL (menggunakan SQL native atau GORM).
3. **Service/Usecase Layer:** Pusat *business logic* (validasi saldo, kalkulasi progres tabungan). Tidak peduli dengan HTTP atau JSON.
4. **Handler/Delivery Layer:** Menangani HTTP Request, parsing JSON, autentikasi, dan memformat HTTP Response (Gin/Fiber layer).

### 2.1 Directory Structure
```text
caupocket-api/
├── cmd/
│   └── api/
│       └── main.go           # Entry point, inisialisasi koneksi DB dan HTTP server
├── internal/                 
│   ├── config/               # Parser file .env (Database URL, JWT Secret)
│   ├── model/                # Struct domain (User, Wallet, Transaction, Goal)
│   ├── repository/           # Eksekusi query database (PostgreSQL)
│   ├── service/              # Business logic dan orkestrasi repository
│   ├── handler/              # HTTP Route handlers dan parsing JSON
│   └── middleware/           # Interceptor (JWT Auth Validator, Logger, CORS)
├── pkg/                      # Utility functions (Bcrypt password, Response Formatter)
├── docs/                     # Auto-generated Swagger documentation
├── .env                      
├── go.mod
└── go.sum
```

---

## 3. Database Schema (Entity-Relationship)
Menggunakan PostgreSQL dengan prinsip integritas tinggi untuk aplikasi finansial.

* **Primary Key:** Menggunakan UUID (menghindari enumerasi URL).
* **Currency:** Menggunakan `BIGINT` untuk mencegah rounding issue pada floating point.
* **Audit Trail:** Menerapkan Soft Deletes (`deleted_at`).

### Tables Definition
* **`users`**: `id` (UUID), `email` (Unique), `password_hash`, `name`, `created_at`.
* **`wallets`**: `id` (UUID), `user_id` (FK), `name`, `balance` (`BIGINT` - Cache saldo), `created_at`.
* **`categories`**: `id` (UUID), `user_id` (FK), `name`, `type` (`INCOME`/`EXPENSE`), `icon_name`.
* **`saving_goals`**: `id` (UUID), `user_id` (FK), `title`, `target_amount` (`BIGINT`), `current_amount` (`BIGINT`), `target_date` (`DATE`), `status` (`ACTIVE`/`ACHIEVED`).
* **`transactions` (Ledger)**:
  * `id` (UUID), `user_id` (FK), `wallet_id` (FK)
  * `category_id` (FK - Nullable)
  * `goal_id` (FK - Nullable)
  * `type` (`INCOME`/`EXPENSE`/`GOAL_FUNDING`)
  * `amount` (`BIGINT` - Selalu absolut/positif)
  * `description` (`TEXT`), `trx_date` (`TIMESTAMP`)

---

## 4. Core Business Logic (The Ledger Engine)
Setiap mutasi dana menggunakan konsep Database Transaction (TX) untuk mencegah data korup jika terjadi kegagalan sistem di tengah proses.

### Cash-Out / Pengeluaran:
1. Insert ke `transactions` (type: `EXPENSE`).
2. Update `balance` di tabel `wallets` (`balance = balance - amount`).

### Goal Funding (Menabung untuk Liburan):
1. Insert ke `transactions` (type: `GOAL_FUNDING`).
2. Update saldo utama di `wallets` berkurang.
3. Update progres tabungan di `saving_goals` bertambah.

> **Note:** Seluruh langkah ini dibungkus dalam 1 blok TX. Jika langkah 3 gagal, langkah 1 dan 2 otomatis di-rollback.

---

## 5. API Design & Contracts
Semua endpoint diawali dengan `/api/v1/` dan didokumentasikan otomatis menggunakan Swagger.

### Standard Response Wrapper

**Success (2xx):**
```json
{
  "success": true,
  "message": "Deskripsi sukses",
  "data": { ... } 
}
```

**Error (4xx / 5xx):**
```json
{
  "success": false,
  "message": "Pesan error manusiawi",
  "error_code": "SPECIFIC_ERROR_CODE"
}
```

### Authentication Strategy
* **Login Flow:** Klien mengirim JSON (`email`, `password`). Backend memvalidasi bcrypt hash.
* **Token Generation:** Backend menghasilkan JWT stateless (menyimpan `user_id` di payload).
* **Delivery:** Token dikembalikan murni dalam format JSON. (Penyimpanan ke dalam cookie adalah tanggung jawab Frontend).
