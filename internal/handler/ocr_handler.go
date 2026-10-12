package handler

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

const maxReceiptSize = 5 << 20 // 5 MB

var allowedReceiptExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// OcrHandler HTTP handler Receipt OCR (EPIC V2.1).
type OcrHandler struct {
	ocr service.OcrService
}

func NewOcrHandler(ocr service.OcrService) *OcrHandler {
	return &OcrHandler{ocr: ocr}
}

// ScanReceipt godoc
// @Summary      Pindai struk belanja (OCR)
// @Description  |
// @Description  Menerima gambar struk (multipart/form-data, field "file") lalu:
// @Description  1. Menjalankan OCR Tesseract (subprocess, PSM 4).
// @Description  2. Mengekstrak nama merchant (baris teratas yang wajar) dan
// @Description  total nominal (baris TOTAL/jumlah/bayar dengan "Rp", abaikan
// @Description  titik/koma pemisah) via regex.
// @Description  3. Mengembalikan DTO untuk auto-fill form transaksi di frontend.
// @Tags         OCR
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "Gambar struk (.jpg/.jpeg/.png/.webp, max 5MB)"
// @Success      200 {object} response.SuccessBody{data=service.OcrReceipt}
// @Failure      400 {object} response.ErrorBody
// @Failure      401 {object} response.ErrorBody
// @Failure      413 {object} response.ErrorBody
// @Failure      422 {object} response.ErrorBody
// @Failure      500 {object} response.ErrorBody
// @Failure      503 {object} response.ErrorBody
// @Router       /ocr/receipt [post]
func (h *OcrHandler) ScanReceipt(c *gin.Context) {
	if _, ok := requireUserID(c); !ok {
		return
	}

	if c.Request.ContentLength > maxReceiptSize+64<<10 {
		response.Error(c, http.StatusRequestEntityTooLarge, "Ukuran gambar maksimal 5MB", response.ErrCodeOcrTooLarge)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Field \"file\" wajib berisi gambar struk", response.ErrCodeOcrFileRequired)
		return
	}
	if fileHeader.Size > maxReceiptSize {
		response.Error(c, http.StatusRequestEntityTooLarge, "Ukuran gambar maksimal 5MB", response.ErrCodeOcrTooLarge)
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedReceiptExt[ext] {
		response.Error(c, http.StatusBadRequest, "Format harus .jpg, .png, atau .webp", response.ErrCodeOcrInvalidType)
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Gagal membaca file", response.ErrCodeOcrFileRequired)
		return
	}
	defer src.Close()

	if !hasImageMagicBytes(src) {
		response.Error(c, http.StatusBadRequest, "Isi file bukan gambar yang valid", response.ErrCodeOcrInvalidType)
		return
	}

	tmp, err := os.CreateTemp("", "caupocket-ocr-*"+ext)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Gagal menyiapkan berkas sementara", response.ErrCodeInternal)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		response.Error(c, http.StatusInternalServerError, "Gagal menyimpan berkas sementara", response.ErrCodeInternal)
		return
	}
	_ = tmp.Close()

	result, err := h.ocr.ScanReceipt(c.Request.Context(), tmpPath)
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "Struk berhasil diproses", result)
}

// hasImageMagicBytes memverifikasi signature JPEG/PNG/WebP dan me-rewind reader.
func hasImageMagicBytes(r io.Reader) bool {
	head := make([]byte, 12)
	n, err := io.ReadFull(r, head)
	if err != nil && n < 12 {
		return false
	}
	if seeker, ok := r.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return false
		}
	}
	png := bytes.HasPrefix(head, []byte{0x89, 'P', 'N', 'G'})
	jpeg := bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF})
	webp := bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP"))
	return png || jpeg || webp
}
