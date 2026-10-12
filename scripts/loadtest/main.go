// loadtest — load generator sederhana untuk Caupocket API.
//
//	go run ./scripts/loadtest -base http://localhost:8080/api/v1 -scenario read -c 25 -dur 15s
//
// Skenario:
//   - read  : campuran GET /me, /wallets, /transactions, /goals (dashboard)
//   - login : POST /auth/login terus-menerus (mengukur throughput bcrypt)
//   - write : POST /transactions INCOME kecil (DB TX + publish event)
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	base = flag.String("base", "http://localhost:8080/api/v1", "base URL API")
	scen = flag.String("scenario", "read", "read | login | write")
	conc = flag.Int("c", 25, "jumlah worker konkuren")
	dur  = flag.Duration("dur", 15*time.Second, "durasi tes")
)

type result struct {
	total   atomic.Int64
	errs    atomic.Int64
	mu      sync.Mutex
	latency []time.Duration
}

func (r *result) record(d time.Duration, ok bool) {
	r.total.Add(1)
	if !ok {
		r.errs.Add(1)
	}
	r.mu.Lock()
	r.latency = append(r.latency, d)
	r.mu.Unlock()
}

func (r *result) report(elapsed time.Duration) {
	r.mu.Lock()
	lats := make([]time.Duration, len(r.latency))
	copy(lats, r.latency)
	r.mu.Unlock()
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		idx := int(float64(len(lats)-1) * p)
		return lats[idx]
	}
	var sum time.Duration
	for _, l := range lats {
		sum += l
	}
	avg := time.Duration(0)
	if len(lats) > 0 {
		avg = sum / time.Duration(len(lats))
	}

	fmt.Printf("\n== HASIL scenario=%s conc=%d dur=%s ==\n", *scen, *conc, dur)
	fmt.Printf("total=%d errors=%d rps=%.1f\n", r.total.Load(), r.errs.Load(), float64(r.total.Load())/elapsed.Seconds())
	fmt.Printf("latency avg=%s p50=%s p95=%s p99=%s max=%s\n\n",
		avg, pct(0.50), pct(0.95), pct(0.99), lats[len(lats)-1])
}

func main() {
	flag.Parse()
	client := &http.Client{Timeout: 10 * time.Second}

	post := func(path string, body any, token string) (int, []byte) {
		raw, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, *base+path, bytes.NewReader(raw))
		if err != nil {
			return 0, nil
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(req)
		if err != nil {
			return 0, nil
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, b
	}
	get := func(path, token string) int {
		req, _ := http.NewRequest(http.MethodGet, *base+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(req)
		if err != nil {
			return 0
		}
		defer res.Body.Close()
		io.Copy(io.Discard, res.Body)
		return res.StatusCode
	}

	// Siapkan user khusus perf (idempotent).
	email := fmt.Sprintf("perf-%d@example.com", time.Now().UnixNano())
	password := "rahasia123"
	post("/auth/register", map[string]any{"name": "Perf", "email": email, "password": password}, "")
	_, body := post("/auth/login", map[string]any{"email": email, "password": password}, "")
	var login struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(body, &login)
	token := login.Data.Token
	if token == "" && *scen != "login" {
		fmt.Println("gagal login user perf:", string(body))
		os.Exit(1)
	}

	var walletID string
	if *scen == "write" {
		req, _ := http.NewRequest(http.MethodGet, *base+"/wallets", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		wbody := []byte(nil)
		if res, err := client.Do(req); err == nil {
			wbody, _ = io.ReadAll(res.Body)
			res.Body.Close()
		}
		var wl struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal(wbody, &wl)
		if len(wl.Data) == 0 {
			fmt.Println("wallet tidak ditemukan")
			os.Exit(1)
		}
		walletID = wl.Data[0].ID
		// Saldo awal agar write tidak pernah kehabisan.
		post("/transactions", map[string]any{"wallet_id": walletID, "type": "INCOME", "amount": 1_000_000_000, "description": "perf topup"}, token)
	}

	reads := []string{"/me", "/wallets", "/transactions?limit=10", "/goals"}
	var rs result
	var wg sync.WaitGroup
	stop := time.After(*dur)

	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			n := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				start := time.Now()
				var ok bool
				switch *scen {
				case "login":
					s, _ := post("/auth/login", map[string]any{"email": email, "password": password}, "")
					ok = s == 200
				case "write":
					s, _ := post("/transactions", map[string]any{
						"wallet_id": walletID, "type": "INCOME", "amount": 1000, "description": "perf write",
					}, token)
					ok = s == 201
				default:
					s := get(reads[n%len(reads)], token)
					ok = s == 200
				}
				n++
				rs.record(time.Since(start), ok)
			}
		}(i)
	}

	start := time.Now()
	wg.Wait()
	rs.report(time.Since(start))
}
