package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"caupocket-api/internal/middleware"
	"caupocket-api/internal/model"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

// WalletHandler HTTP handler dompet.
type WalletHandler struct {
	wallets service.WalletService
}

func NewWalletHandler(wallets service.WalletService) *WalletHandler {
	return &WalletHandler{wallets: wallets}
}

// ListWallets godoc
// @Summary      Daftar dompet beserta saldo
// @Description  Frontend menjumlahkan balance seluruh dompet untuk menampilkan total saldo.
// @Tags         Wallets
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} response.SuccessBody{data=[]model.Wallet}
// @Failure      401 {object} response.ErrorBody
// @Router       /wallets [get]
func (h *WalletHandler) ListWallets(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	wallets, err := h.wallets.ListByUser(c.Request.Context(), userID)
	if err != nil {
		abortServiceError(c, err)
		return
	}
	if wallets == nil {
		wallets = []model.Wallet{}
	}
	response.Success(c, http.StatusOK, "OK", wallets)
}
