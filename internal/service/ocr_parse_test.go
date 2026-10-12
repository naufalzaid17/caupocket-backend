package service

import (
	"testing"
)

func strPtrOK(t *testing.T, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("hasil nil, ingin %q", want)
	}
	if *got != want {
		t.Fatalf("hasil = %q, ingin %q", *got, want)
	}
}

func intPtrOK(t *testing.T, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("hasil nil, ingin %d", want)
	}
	if *got != want {
		t.Fatalf("hasil = %d, ingin %d", *got, want)
	}
}

func TestParseRealisticReceipt(t *testing.T) {
	raw := `
WARUNG MAKAN BAHAGIA
Jl. Merdeka No. 12, Bandung
--------------------------------
12/10/2026 14:32
No: 0081
--------------------------------
Nasi Goreng x1 25.000
Es Teh Manis x2 10.000
Kerupuk x1 12.500
--------------------------------
SUBTOTAL 47.500
PB1 11% 5.225
TOTAL 52.725
TUNAI 100.000
KEMBALI 47.275
Terima kasih!
`
	r := parseReceiptText(raw)

	strPtrOK(t, r.MerchantName, "WARUNG MAKAN BAHAGIA")
	intPtrOK(t, r.TotalAmount, 52725)
	strPtrOK(t, r.Date, "2026-10-12")
}

func TestParsePrefersGrandTotalOverRpLines(t *testing.T) {
	// Tanpa baris TOTAL — fallback ke baris Rp terbesar.
	raw := `
KOPI KENANGAN SENJA
Rp 18.000
Rp 32.500
Tunai Rp 50.000
`
	r := parseReceiptText(raw)
	strPtrOK(t, r.MerchantName, "KOPI KENANGAN SENJA")
	intPtrOK(t, r.TotalAmount, 50000)
}

func TestParseSkipsSubtotal(t *testing.T) {
	raw := `
TOKO SERBA ADA
SUBTOTAL 40.000
TOTAL 44.000
`
	r := parseReceiptText(raw)
	intPtrOK(t, r.TotalAmount, 44000)
}

func TestParseDateVariants(t *testing.T) {
	cases := map[string]string{
		"Struk\n01-12-2026 TOTAL 10.000": "2026-12-01",
		"Struk\n2026-03-05 TOTAL 10.000": "2026-03-05",
		"Struk\n5.3.26 TOTAL 10.000":     "2026-03-05",
		"Struk\n05/03/2026 TOTAL 10.000": "2026-03-05",
	}
	for raw, want := range cases {
		r := parseReceiptText(raw)
		if r.Date == nil {
			t.Fatalf("tanggal tidak terbaca dari %q", raw)
		}
		if *r.Date != want {
			t.Fatalf("tanggal dari %q = %q, ingin %q", raw, *r.Date, want)
		}
	}

	// Nominal berformat ribuan tidak boleh salah dibaca sebagai tanggal.
	r := parseReceiptText("TOKO ABC\nTOTAL 1.234.567")
	if r.Date != nil && *r.Date != "" {
		t.Fatalf("1.234.567 salah terbaca sebagai tanggal: %q", *r.Date)
	}
}

func TestSanitizeAmount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"45.000", 45000, true},
		{"1.234.567", 1234567, true},
		{"15.000,00", 15000, true},
		{"25000", 25000, true},
		{"45,000.00", 45000, true},
		{"12.500", 12500, true},
		{"45.00", 45, true},
		{"45 000", 45000, true},
		{"0", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"999999999999999999999", 0, false}, // overflow → tolak
	}
	for _, c := range cases {
		got, ok := sanitizeAmount(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("sanitizeAmount(%q) = (%d, %v), ingin (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseMerchantHeuristics(t *testing.T) {
	// Baris pertama adalah header generik → merchant di baris kedua.
	r := parseReceiptText("STRUK\nBAKMI GM PAKUBUWONI\nTOTAL 75.000")
	strPtrOK(t, r.MerchantName, "BAKMI GM PAKUBUWONI")

	// Baris pertama penuh angka (nomor order) → dilewati.
	r = parseReceiptText("ORDER #12345\nMARTABAK SPESIAL BANGKA\nTOTAL 88.000")
	strPtrOK(t, r.MerchantName, "MARTABAK SPESIAL BANGKA")

	// Tidak ada baris yang wajar → merchant nil.
	r = parseReceiptText("123456\nwww.contoh.id\nTOTAL 10.000")
	if r.MerchantName != nil {
		t.Fatalf("merchant seharusnya nil, dapat %q", *r.MerchantName)
	}
}
