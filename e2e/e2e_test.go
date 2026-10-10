//go:build e2e

// Test end-to-end terhadap HTTP layer penuh (router + middleware + service +
// repository + PostgreSQL sungguhan via embedded-postgres).
//
// Jalankan dengan:  go test -tags e2e ./e2e
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"

	"caupocket-api/internal/app"
	"caupocket-api/internal/config"
)

type envelope struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	ErrorCode string          `json:"error_code"`
}

func doRequest(t *testing.T, ts *httptest.Server, method, path, token string, body any) (int, envelope) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body gagal: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatalf("membuat request gagal: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request %s %s gagal: %v", method, path, err)
	}
	defer res.Body.Close()

	var env envelope
	raw, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(raw, &env); err != nil && len(raw) > 0 {
		t.Fatalf("response bukan JSON valid (%s %s): %s", method, path, raw)
	}
	return res.StatusCode, env
}

func decodeData(t *testing.T, env envelope, target any) {
	t.Helper()
	if err := json.Unmarshal(env.Data, target); err != nil {
		t.Fatalf("gagal decode data: %v (raw: %s)", err, env.Data)
	}
}

func TestLedgerEndToEnd(t *testing.T) {
	// --- Bootstrap embedded PostgreSQL -------------------------------------
	basePath := filepath.Join(os.TempDir(), "caupocket-e2e-pg")
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		t.Fatalf("mkdir gagal: %v", err)
	}
	pg := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Port(5433).
			Username("caupocket").Password("caupocket").Database("caupocket").
			BinariesPath(filepath.Join(basePath, "bin")).
			DataPath(filepath.Join(basePath, "data")).
			RuntimePath(filepath.Join(basePath, "runtime")),
	)
	if err := pg.Start(); err != nil {
		t.Fatalf("embedded postgres gagal start: %v", err)
	}
	defer func() {
		if err := pg.Stop(); err != nil {
			t.Logf("embedded postgres stop: %v", err)
		}
	}()

	cfg := &config.Config{
		Port:         "0",
		DatabaseURL:  "postgres://caupocket:caupocket@localhost:5433/caupocket?sslmode=disable&client_encoding=UTF8",
		JWTSecret:    "e2e-test-secret-0123456789abcdef",
		JWTExpiresIn: time.Hour,
		GinMode:      "test",
	}
	router, cleanup, err := app.Build(cfg)
	if err != nil {
		t.Fatalf("app.Build gagal: %v", err)
	}
	defer cleanup()

	ts := httptest.NewServer(router)
	defer ts.Close()

	email := fmt.Sprintf("naufal-%d@example.com", time.Now().UnixNano())
	password := "rahasia123"

	// --- Health & swagger ---------------------------------------------------
	if code, _ := doRequest(t, ts, "GET", "/healthz", "", nil); code != 200 {
		t.Fatalf("healthz = %d, ingin 200", code)
	}
	swaggerRes, err := ts.Client().Get(ts.URL + "/swagger/index.html")
	if err != nil || swaggerRes.StatusCode != 200 {
		t.Fatalf("swagger UI gagal: status=%v err=%v", swaggerRes, err)
	}
	raw, _ := io.ReadAll(swaggerRes.Body)
	swaggerRes.Body.Close()
	if !bytes.Contains(raw, []byte("swagger-ui")) {
		t.Fatalf("swagger UI tidak menyajikan UI: %s", raw[:min(len(raw), 200)])
	}

	// --- Register ------------------------------------------------------------
	code, env := doRequest(t, ts, "POST", "/api/v1/auth/register", "", map[string]any{
		"name": "Naufal Tester", "email": email, "password": password,
	})
	if code != 201 {
		t.Fatalf("register = %d (%s %s), ingin 201", code, env.Message, env.ErrorCode)
	}

	// Duplicate email ditolak.
	if code, env := doRequest(t, ts, "POST", "/api/v1/auth/register", "", map[string]any{
		"name": "Duplikat", "email": email, "password": password,
	}); code != 409 || env.ErrorCode != "EMAIL_ALREADY_EXISTS" {
		t.Fatalf("register duplikat = %d (%s), ingin 409 EMAIL_ALREADY_EXISTS", code, env.ErrorCode)
	}

	// --- Login ----------------------------------------------------------------
	if code, _ := doRequest(t, ts, "POST", "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": "password-ngacoo",
	}); code != 401 {
		t.Fatalf("login salah password = %d, ingin 401", code)
	}

	code, env = doRequest(t, ts, "POST", "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": password,
	})
	if code != 200 {
		t.Fatalf("login = %d (%s), ingin 200", code, env.Message)
	}
	var loginData struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	decodeData(t, env, &loginData)
	if loginData.Token == "" || loginData.User.ID == "" {
		t.Fatalf("login tidak mengembalikan token/user: %s", env.Data)
	}
	token := loginData.Token

	// Tanpa token → 401; dengan token → 200.
	if code, _ := doRequest(t, ts, "GET", "/api/v1/me", "", nil); code != 401 {
		t.Fatalf("me tanpa token = %d, ingin 401", code)
	}
	if code, _ := doRequest(t, ts, "GET", "/api/v1/me", "token-ngawur", nil); code != 401 {
		t.Fatalf("me token ngawur = %d, ingin 401", code)
	}
	if code, _ := doRequest(t, ts, "GET", "/api/v1/me", token, nil); code != 200 {
		t.Fatalf("me = %d, ingin 200", code)
	}

	// --- Wallet default ---------------------------------------------------
	type wallet struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Balance int64  `json:"balance"`
	}
	code, env = doRequest(t, ts, "GET", "/api/v1/wallets", token, nil)
	if code != 200 {
		t.Fatalf("wallets = %d, ingin 200", code)
	}
	var wallets []wallet
	decodeData(t, env, &wallets)
	if len(wallets) != 1 || wallets[0].Name != "Dompet Utama" || wallets[0].Balance != 0 {
		t.Fatalf("dompet default tidak sesuai: %+v", wallets)
	}
	walletID := wallets[0].ID

	getBalance := func() int64 {
		_, env := doRequest(t, ts, "GET", "/api/v1/wallets", token, nil)
		var ws []wallet
		decodeData(t, env, &ws)
		for _, w := range ws {
			if w.ID == walletID {
				return w.Balance
			}
		}
		t.Fatal("dompet hilang")
		return 0
	}

	// --- INCOME 1.000.000 ---------------------------------------------------
	code, env = doRequest(t, ts, "POST", "/api/v1/transactions", token, map[string]any{
		"wallet_id": walletID, "type": "INCOME", "amount": 1_000_000,
		"description": "Gaji bulan Oktober", "trx_date": "2026-10-01T09:00:00Z",
	})
	if code != 201 {
		t.Fatalf("income = %d (%s %s), ingin 201", code, env.Message, env.ErrorCode)
	}
	if b := getBalance(); b != 1_000_000 {
		t.Fatalf("saldo setelah income = %d, ingin 1000000", b)
	}

	// --- EXPENSE (cash-out) 250.000 ----------------------------------------
	code, env = doRequest(t, ts, "POST", "/api/v1/transactions", token, map[string]any{
		"wallet_id": walletID, "type": "EXPENSE", "amount": 250_000,
		"description": "Makan siang",
	})
	if code != 201 {
		t.Fatalf("expense = %d (%s %s), ingin 201", code, env.Message, env.ErrorCode)
	}
	if b := getBalance(); b != 750_000 {
		t.Fatalf("saldo setelah expense = %d, ingin 750000", b)
	}

	// --- Cash-out melebihi saldo → ditolak, saldo utuh ---------------------
	code, env = doRequest(t, ts, "POST", "/api/v1/transactions", token, map[string]any{
		"wallet_id": walletID, "type": "EXPENSE", "amount": 999_999,
		"description": "Belanja besar",
	})
	if code != 422 || env.ErrorCode != "INSUFFICIENT_BALANCE" {
		t.Fatalf("expense overdraft = %d (%s), ingin 422 INSUFFICIENT_BALANCE", code, env.ErrorCode)
	}
	if b := getBalance(); b != 750_000 {
		t.Fatalf("saldo berubah setelah transaksi ditolak: %d, ingin 750000", b)
	}

	// Amount tidak boleh nol/negatif.
	if code, _ := doRequest(t, ts, "POST", "/api/v1/transactions", token, map[string]any{
		"wallet_id": walletID, "type": "EXPENSE", "amount": 0,
	}); code != 400 {
		t.Fatalf("amount 0 = %d, ingin 400", code)
	}

	// GOAL_FUNDING tidak boleh lewat endpoint transactions.
	if code, _ := doRequest(t, ts, "POST", "/api/v1/transactions", token, map[string]any{
		"wallet_id": walletID, "type": "GOAL_FUNDING", "amount": 100,
	}); code != 400 {
		t.Fatalf("GOAL_FUNDING via /transactions = %d, ingin 400", code)
	}

	// --- Buat goal -----------------------------------------------------------
	code, env = doRequest(t, ts, "POST", "/api/v1/goals", token, map[string]any{
		"title": "Liburan ke Bali", "target_amount": 5_000_000, "target_date": "2026-12-20",
	})
	if code != 201 {
		t.Fatalf("create goal = %d (%s), ingin 201", code, env.Message)
	}
	var goal struct {
		ID            string `json:"id"`
		CurrentAmount int64  `json:"current_amount"`
		Status        string `json:"status"`
	}
	decodeData(t, env, &goal)
	if goal.Status != "ACTIVE" {
		t.Fatalf("status goal baru = %s, ingin ACTIVE", goal.Status)
	}

	// --- Goal funding 500.000 -----------------------------------------------
	code, env = doRequest(t, ts, "POST", "/api/v1/goals/"+goal.ID+"/fund", token, map[string]any{
		"wallet_id": walletID, "amount": 500_000,
	})
	if code != 201 {
		t.Fatalf("fund goal = %d (%s %s), ingin 201", code, env.Message, env.ErrorCode)
	}
	if b := getBalance(); b != 250_000 {
		t.Fatalf("saldo setelah funding = %d, ingin 250000", b)
	}

	var goals []struct {
		ID            string `json:"id"`
		CurrentAmount int64  `json:"current_amount"`
		Status        string `json:"status"`
	}
	_, env = doRequest(t, ts, "GET", "/api/v1/goals", token, nil)
	decodeData(t, env, &goals)
	if len(goals) != 1 || goals[0].CurrentAmount != 500_000 {
		t.Fatalf("current_amount goal = %+v, ingin 500000", goals)
	}

	// --- Funding melebihi saldo → rollback penuh ---------------------------
	code, env = doRequest(t, ts, "POST", "/api/v1/goals/"+goal.ID+"/fund", token, map[string]any{
		"wallet_id": walletID, "amount": 300_000,
	})
	if code != 422 || env.ErrorCode != "INSUFFICIENT_BALANCE" {
		t.Fatalf("funding overdraft = %d (%s), ingin 422 INSUFFICIENT_BALANCE", code, env.ErrorCode)
	}
	if b := getBalance(); b != 250_000 {
		t.Fatalf("saldo berubah setelah funding ditolak: %d, ingin 250000", b)
	}
	_, env = doRequest(t, ts, "GET", "/api/v1/goals", token, nil)
	decodeData(t, env, &goals)
	if goals[0].CurrentAmount != 500_000 {
		t.Fatalf("current_amount berubah setelah funding ditolak: %+v (TX tidak rollback?)", goals)
	}

	// --- Top up besar lalu capai target → ACHIEVED -------------------------
	if code, _ := doRequest(t, ts, "POST", "/api/v1/transactions", token, map[string]any{
		"wallet_id": walletID, "type": "INCOME", "amount": 5_000_000, "description": "Bonus",
	}); code != 201 {
		t.Fatalf("income kedua = %d, ingin 201", code)
	}
	if code, _ := doRequest(t, ts, "POST", "/api/v1/goals/"+goal.ID+"/fund", token, map[string]any{
		"wallet_id": walletID, "amount": 4_500_000,
	}); code != 201 {
		t.Fatalf("funding penutup = %d, ingin 201", code)
	}
	_, env = doRequest(t, ts, "GET", "/api/v1/goals", token, nil)
	decodeData(t, env, &goals)
	if goals[0].Status != "ACHIEVED" || goals[0].CurrentAmount != 5_000_000 {
		t.Fatalf("goal seharusnya ACHIEVED 5000000: %+v", goals)
	}
	if b := getBalance(); b != 750_000 {
		t.Fatalf("saldo akhir = %d, ingin 750000", b)
	}

	// Goal tercapai tidak bisa difund lagi.
	if code, env := doRequest(t, ts, "POST", "/api/v1/goals/"+goal.ID+"/fund", token, map[string]any{
		"wallet_id": walletID, "amount": 10_000,
	}); code != 409 || env.ErrorCode != "GOAL_ALREADY_ACHIEVED" {
		t.Fatalf("funding goal tercapai = %d (%s), ingin 409 GOAL_ALREADY_ACHIEVED", code, env.ErrorCode)
	}

	// --- Ledger list ---------------------------------------------------------
	type trx struct {
		Type        string `json:"type"`
		Amount      int64  `json:"amount"`
		WalletName  string `json:"wallet_name"`
		Description string `json:"description"`
	}
	_, env = doRequest(t, ts, "GET", "/api/v1/transactions?limit=50", token, nil)
	var transactions []trx
	decodeData(t, env, &transactions)

	counts := map[string]int{}
	for _, tx := range transactions {
		counts[tx.Type]++
		if tx.WalletName != "Dompet Utama" {
			t.Fatalf("wallet_name tidak terisi: %+v", tx)
		}
	}
	if counts["INCOME"] != 2 || counts["EXPENSE"] != 1 || counts["GOAL_FUNDING"] != 2 {
		t.Fatalf("komposisi ledger tidak sesuai: %+v", counts)
	}

	// --- Kategori ------------------------------------------------------------
	_, env = doRequest(t, ts, "GET", "/api/v1/categories", token, nil)
	var categories []map[string]any
	decodeData(t, env, &categories)
	if len(categories) < 6 {
		t.Fatalf("kategori default tidak ter-seed: %d", len(categories))
	}

	t.Log("SEMUA SKENARIO LEDGER LOLOS ✔")
}
