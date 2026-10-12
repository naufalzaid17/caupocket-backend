package service

// Parser raw text struk hasil OCR (EPIC V2.1).
// Murni fungsi string tanpa dependensi eksternal sehingga mudah di-unit-test.

import (
	"regexp"
	"strconv"
	"strings"
)

// OcrReceipt DTO hasil ekstraksi struk untuk auto-fill form transaksi.
type OcrReceipt struct {
	MerchantName *string `json:"merchant_name" example:"Warung Makan Bahagia"`
	TotalAmount  *int64  `json:"total_amount" example:"52725"`
	Date         *string `json:"date" example:"2026-10-12"`
	RawText      string  `json:"raw_text"`
}

var (
	// Nominal: opsional "Rp"/"IDR" diikuti deretan digit dengan pemisah titik/koma/spasi.
	amountPattern = regexp.MustCompile(`(?i)(?:rp\.?|idr)?\s*([0-9][0-9.,\s]*[0-9]|[0-9])`)
	// Tanggal: yyyy-mm-dd ATAU dd-mm-yy(yy) dengan pemisah - / .
	datePattern = regexp.MustCompile(`\b(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})\b|\b(\d{1,2})[-/.](\d{1,2})[-/.](\d{2,4})\b`)
	onlyDigits  = regexp.MustCompile(`^[0-9]+$`)
)

// Kata yang menandai baris total (sub-total sengaja dikecualikan).
func isTotalLine(lower string) bool {
	if strings.Contains(lower, "subtotal") || strings.Contains(lower, "sub-total") {
		return false
	}
	return strings.Contains(lower, "total") ||
		strings.Contains(lower, "jumlah") ||
		strings.Contains(lower, "bayar")
}

// kata kunci header struk yang bukan nama merchant.
var nonMerchantHeaders = map[string]bool{
	"struk": true, "nota": true, "kwitansi": true, "invoice": true, "receipt": true,
}

// parseReceiptText mengekstrak MerchantName (baris teratas yang wajar),
// TotalAmount (baris TOTAL/jumlah/bayar; fallback baris ber-Rp), dan Date.
func parseReceiptText(raw string) OcrReceipt {
	var lines []string
	for _, l := range strings.Split(raw, "\n") {
		l = strings.Join(strings.Fields(l), " ") // rapikan spasi ganda
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}

	result := OcrReceipt{RawText: strings.TrimSpace(raw)}
	if len(lines) == 0 {
		return result
	}

	result.MerchantName = findMerchant(lines)
	result.TotalAmount = findTotal(lines)
	result.Date = findDate(lines)
	return result
}

// findMerchant: baris pertama dari ~6 baris teratas yang didominasi huruf,
// bukan angka/tanggal/telepon/URL, dan bukan kata header generik.
func findMerchant(lines []string) *string {
	limit := 6
	if len(lines) < limit {
		limit = len(lines)
	}
	for _, line := range lines[:limit] {
		letters, digits := 0, 0
		for _, r := range line {
			switch {
			case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
				letters++
			case r >= '0' && r <= '9':
				digits++
			}
		}
		lower := strings.ToLower(strings.TrimRight(line, ".,-:;"))
		if letters < 3 || digits >= letters {
			continue // angka dominan: tanggal/telepon/harga/nomor order
		}
		if strings.HasPrefix(line, "+") || strings.Contains(lower, "www.") || strings.Contains(lower, "http") {
			continue
		}
		if nonMerchantHeaders[lower] {
			continue
		}
		name := strings.Trim(line, " .,-:;")
		return &name
	}
	return nil
}

// findTotal: ambil nominal TERAKHIR pada tiap baris kandidat, lalu pilih
// nominal terbesar (grand total lazimnya paling besar).
func findTotal(lines []string) *int64 {
	var keywordCandidates, rpCandidates []int64
	for _, line := range lines {
		amount, ok := lastAmount(line)
		if !ok {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case isTotalLine(lower):
			keywordCandidates = append(keywordCandidates, amount)
		case strings.Contains(lower, "rp") || strings.Contains(lower, "idr"):
			rpCandidates = append(rpCandidates, amount)
		}
	}

	pool := keywordCandidates
	if len(pool) == 0 {
		pool = rpCandidates
	}
	var best *int64
	for _, v := range pool {
		v := v
		if best == nil || v > *best {
			best = &v
		}
	}
	return best
}

// lastAmount mengembalikan nominal terakhir yang valid pada sebuah baris
// (total biasanya tertulis di ujung kanan baris).
func lastAmount(line string) (int64, bool) {
	matches := amountPattern.FindAllStringSubmatch(line, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		if v, ok := sanitizeAmount(matches[i][1]); ok {
			return v, true
		}
	}
	return 0, false
}

// sanitizeAmount normalisasi string nominal menjadi int64 rupiah utuh.
// Aturan: titik = ribuan (gaya Indonesia), koma = desimal; pemisah dibuang.
func sanitizeAmount(s string) (int64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if s == "" || !strings.ContainsAny(s, "0123456789") {
		return 0, false
	}

	dot := strings.LastIndex(s, ".")
	comma := strings.LastIndex(s, ",")

	switch {
	case dot >= 0 && comma >= 0:
		// Separator paling akhir dianggap desimal bila diikuti 1-2 digit.
		last := max(dot, comma)
		if len(s)-last-1 <= 2 {
			s = s[:last]
		}
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", "")
	case comma >= 0:
		if len(s)-comma-1 <= 2 {
			s = s[:comma] // "15.000,00" → "15.000"
		}
		s = strings.ReplaceAll(s, ",", "")
	case dot >= 0:
		parts := strings.Split(s, ".")
		thousands := len(parts) > 1
		for _, p := range parts[1:] {
			if len(p) != 3 {
				thousands = false
				break
			}
		}
		if !thousands {
			s = s[:dot] // desimal ala "45.00" / "45.5"
		}
		s = strings.ReplaceAll(s, ".", "")
	}

	if !onlyDigits.MatchString(s) || len(s) > 15 {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// findDate mencari tanggal valid pertama, dinormalisasi ke YYYY-MM-DD.
func findDate(lines []string) *string {
	for _, line := range lines {
		for _, m := range datePattern.FindAllStringSubmatch(line, -1) {
			var y, mo, d string
			if m[1] != "" {
				y, mo, d = m[1], m[2], m[3]
			} else {
				d, mo, y = m[4], m[5], m[6]
			}
			if len(y) == 2 {
				y = "20" + y
			}
			moi, errM := strconv.Atoi(mo)
			di, errD := strconv.Atoi(d)
			yi, errY := strconv.Atoi(y)
			if errM != nil || errD != nil || errY != nil {
				continue
			}
			if moi < 1 || moi > 12 || di < 1 || di > 31 || yi < 2000 || yi > 2100 {
				continue
			}
			out := y + "-" + pad2(mo) + "-" + pad2(d)
			return &out
		}
	}
	return nil
}

func pad2(v string) string {
	if len(v) == 1 {
		return "0" + v
	}
	return v
}
