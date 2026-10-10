package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"caupocket-api/internal/middleware"
	"caupocket-api/internal/model"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

type createTransactionRequest struct {
	WalletID    string  `json:"wallet_id" binding:"required,uuid" example:"0f2b1a3c-1111-4a2b-8c3d-000000000001"`
	CategoryID  *string `json:"category_id" binding:"omitempty,uuid" example:"0f2b1a3c-1111-4a2b-8c3d-000000000002"`
	Type        string  `json:"type" binding:"required,oneof=INCOME EXPENSE" example:"EXPENSE"`
	Amount      int64   `json:"amount" binding:"required,gt=0" example:"25000"`
	Description string  `json:"description" binding:"max=500" example:"Makan siang di warung"`
	TrxDate     string  `json:"trx_date" example:"2026-10-01T09:00:00Z"`
}

type transactionResponse struct {
	ID           uuid.UUID    `json:"id"`
	Type         string       `json:"type"`
	Amount       int64        `json:"amount"`
	Description  string       `json:"description"`
	TrxDate      time.Time    `json:"trx_date"`
	WalletID     uuid.UUID    `json:"wallet_id"`
	WalletName   string       `json:"wallet_name"`
	CategoryID   *uuid.UUID   `json:"category_id"`
	CategoryName *string      `json:"category_name"`
	GoalID       *uuid.UUID   `json:"goal_id"`
	GoalTitle    *string      `json:"goal_title"`
}

func toTransactionResponse(t *model.Transaction) transactionResponse {
	resp := transactionResponse{
		ID:          t.ID,
		Type:        string(t.Type),
		Amount:      t.Amount,
		Description: t.Description,
		TrxDate:     t.TrxDate,
		WalletID:    t.WalletID,
		WalletName:  "-",
		CategoryID:  t.CategoryID,
		GoalID:      t.GoalID,
	}
	if t.Wallet != nil {
		resp.WalletName = t.Wallet.Name
	}
	if t.Category != nil {
		name := t.Category.Name
		resp.CategoryName = &name
	}
	if t.Goal != nil {
		title := t.Goal.Title
		resp.GoalTitle = &title
	}
	return resp
}

type fundGoalRequest struct {
	WalletID string `json:"wallet_id" binding:"required,uuid" example:"0f2b1a3c-1111-4a2b-8c3d-000000000001"`
	Amount   int64  `json:"amount" binding:"required,gt=0" example:"500000"`
}

type fundGoalData struct {
	Transaction transactionResponse `json:"transaction"`
	Goal        model.SavingGoal    `json:"goal"`
}

// TransactionHandler HTTP handler ledger (transaksi & goal funding).
type TransactionHandler struct {
	transactions service.TransactionService
}

func NewTransactionHandler(transactions service.TransactionService) *TransactionHandler {
	return &TransactionHandler{transactions: transactions}
}

// CreateTransaction godoc
// @Summary      Catat transaksi (income / cash-out)
// @Description  |
// @Description  Mencatat transaksi sekaligus memutasi saldo dompet dalam SATU database transaction:
// @Description  * INCOME: saldo dompet bertambah.
// @Description  * EXPENSE (cash-out): saldo divalidasi terlebih dahulu; bila kurang, request ditolak (422) dan tidak ada perubahan.
// @Description  Tipe GOAL_FUNDING tidak diizinkan di sini — gunakan endpoint /goals/{id}/fund.
// @Tags         Transactions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body createTransactionRequest true "Payload transaksi"
// @Success      201 {object} response.SuccessBody{data=transactionResponse}
// @Failure      400 {object} response.ErrorBody
// @Failure      401 {object} response.ErrorBody
// @Failure      404 {object} response.ErrorBody
// @Failure      422 {object} response.ErrorBody
// @Failure      500 {object} response.ErrorBody
// @Router       /transactions [post]
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	var req createTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	walletID, err := uuid.Parse(req.WalletID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "wallet_id bukan UUID valid", response.ErrCodeValidation)
		return
	}

	var categoryID *uuid.UUID
	if req.CategoryID != nil && *req.CategoryID != "" {
		id, err := uuid.Parse(*req.CategoryID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "category_id bukan UUID valid", response.ErrCodeValidation)
			return
		}
		categoryID = &id
	}

	trxDate, err := parseFlexibleTime(req.TrxDate)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	trx, err := h.transactions.CreateTransaction(c.Request.Context(), userID, service.CreateTransactionInput{
		WalletID:    walletID,
		CategoryID:  categoryID,
		Type:        model.TransactionType(req.Type),
		Amount:      req.Amount,
		Description: req.Description,
		TrxDate:     trxDate,
	})
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "Transaksi berhasil dicatat", toTransactionResponse(trx))
}

// ListTransactions godoc
// @Summary      Daftar transaksi terakhir
// @Tags         Transactions
// @Security     BearerAuth
// @Produce      json
// @Param        limit query int false "Jumlah maksimal data (default 10, max 100)" default(10)
// @Success      200 {object} response.SuccessBody{data=[]transactionResponse}
// @Failure      401 {object} response.ErrorBody
// @Router       /transactions [get]
func (h *TransactionHandler) ListTransactions(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	limit := 10
	if v := c.Query("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 || parsed > 100 {
			response.Error(c, http.StatusBadRequest, "limit harus angka 1-100", response.ErrCodeValidation)
			return
		}
		limit = parsed
	}

	transactions, err := h.transactions.ListByUser(c.Request.Context(), userID, limit)
	if err != nil {
		abortServiceError(c, err)
		return
	}

	data := make([]transactionResponse, 0, len(transactions))
	for i := range transactions {
		data = append(data, toTransactionResponse(&transactions[i]))
	}
	response.Success(c, http.StatusOK, "OK", data)
}

// FundGoal godoc
// @Summary      Setor dana ke target tabungan (goal funding)
// @Description  |
// @Description  Memindahkan dana dari dompet ke saving_goals dalam SATU database transaction:
// @Description  1. Validasi saldo dompet (kunci baris SELECT ... FOR UPDATE).
// @Description  2. Kurangi balance dompet.
// @Description  3. Tambah current_amount goal (status otomatis ACHIEVED bila target tercapai).
// @Description  4. Catat transaksi tipe GOAL_FUNDING.
// @Description  Jika salah satu langkah gagal, seluruh mutasi di-rollback.
// @Tags         Goals
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path string true "Goal ID (UUID)" example:"0f2b1a3c-2222-4a2b-8c3d-000000000003"
// @Param        request body fundGoalRequest true "Payload setor tabungan"
// @Success      201 {object} response.SuccessBody{data=fundGoalData}
// @Failure      400 {object} response.ErrorBody
// @Failure      401 {object} response.ErrorBody
// @Failure      404 {object} response.ErrorBody
// @Failure      409 {object} response.ErrorBody
// @Failure      422 {object} response.ErrorBody
// @Router       /goals/{id}/fund [post]
func (h *TransactionHandler) FundGoal(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	goalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id goal bukan UUID valid", response.ErrCodeValidation)
		return
	}

	var req fundGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	walletID, err := uuid.Parse(req.WalletID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "wallet_id bukan UUID valid", response.ErrCodeValidation)
		return
	}

	result, err := h.transactions.FundGoal(c.Request.Context(), userID, service.FundGoalInput{
		GoalID:   goalID,
		WalletID: walletID,
		Amount:   req.Amount,
	})
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "Setoran tabungan berhasil", fundGoalData{
		Transaction: toTransactionResponse(result.Transaction),
		Goal:        *result.Goal,
	})
}
