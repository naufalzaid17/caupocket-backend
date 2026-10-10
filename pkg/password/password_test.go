package password

import (
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("rahasia123")
	if err != nil {
		t.Fatalf("HashPassword gagal: %v", err)
	}
	if hash == "rahasia123" || len(hash) < 20 {
		t.Fatalf("hash tidak terlihat seperti bcrypt: %q", hash)
	}
	if !CheckPasswordHash(hash, "rahasia123") {
		t.Fatal("password benar seharusnya lolos validasi")
	}
	if CheckPasswordHash(hash, "password-salah") {
		t.Fatal("password salah seharusnya gagal validasi")
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, _ := HashPassword("sama-sama")
	h2, _ := HashPassword("sama-sama")
	if h1 == h2 {
		t.Fatal("dua hash untuk password sama harus berbeda (salt acak)")
	}
	if strings.HasPrefix(h1, "$2a$") != true {
		t.Fatalf("format bcrypt tidak dikenali: %q", h1)
	}
}
