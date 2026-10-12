package service

// OCR Service (EPIC V2.1): menjalankan binary Tesseract sebagai subprocess.
// Pendekatan tanpa cgo ini membuat binary Go tetap mudah di-build lintas
// platform — cukup pastikan Tesseract terpasang di host.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os/exec"
	"strings"
	"time"
)

// OcrService abstraksi engine OCR agar mudah diganti (mis. Cloud Vision).
type OcrService interface {
	ScanReceipt(ctx context.Context, imagePath string) (*OcrReceipt, error)
}

type ocrService struct {
	bin     string
	lang    string
	timeout time.Duration
}

func NewOcrService(bin, lang string, timeout time.Duration) OcrService {
	return &ocrService{bin: bin, lang: lang, timeout: timeout}
}

// ScanReceipt menjalankan `tesseract <image> stdout -l <lang> --psm 4`
// (psm 4 = kolom tunggal ukuran variabel, cocok untuk struk) lalu mem-parsing
// hasilnya dengan regex.
func (s *ocrService) ScanReceipt(ctx context.Context, imagePath string) (*OcrReceipt, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// #nosec G204 -- path binary berasal dari konfigurasi operator (TESSERACT_BIN),
	// argumen fixed, dan exec.Command tidak melewati shell.
	cmd := exec.CommandContext(ctx, s.bin, imagePath, "stdout", "-l", s.lang, "--psm", "4")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		stderrLower := strings.ToLower(stderr.String())
		switch {
		case errors.Is(err, exec.ErrNotFound) ||
			strings.Contains(stderrLower, "not recognized") ||
			strings.Contains(stderrLower, "no such file"):
			return nil, ErrOcrUnavailable
		case ctx.Err() == context.DeadlineExceeded:
			log.Printf("OCR timeout setelah %s: %v", s.timeout, err)
			return nil, ErrOcrFailed
		default:
			log.Printf("OCR gagal: %v (stderr: %s)", err, stderr.String())
			return nil, ErrOcrFailed
		}
	}

	raw := string(out)
	if strings.TrimSpace(raw) == "" {
		return nil, ErrOcrNoText
	}

	result := parseReceiptText(raw)
	return &result, nil
}
