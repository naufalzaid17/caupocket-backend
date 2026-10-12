package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"caupocket-api/internal/model"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

type habitSettingsResponse struct {
	Enabled      bool  `json:"enabled"`
	StreakDays   int   `json:"streak_days"`
	RewardAmount int64 `json:"reward_amount"`
}

type updateHabitSettingsRequest struct {
	Enabled bool `json:"enabled"`
}

// HabitHandler HTTP handler konfigurasi & riwayat habit engine (EPIC V2.2).
type HabitHandler struct {
	habits service.HabitService
}

func NewHabitHandler(habits service.HabitService) *HabitHandler {
	return &HabitHandler{habits: habits}
}

// GetSettings godoc
// @Summary      Status fitur auto-nabung (habit engine)
// @Description  Mengembalikan status toggle auto-nabung user beserta parameter aturan (streak & reward).
// @Tags         Habits
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} response.SuccessBody{data=habitSettingsResponse}
// @Failure      401 {object} response.ErrorBody
// @Router       /habits/settings [get]
func (h *HabitHandler) GetSettings(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	setting, err := h.habits.GetSettings(c.Request.Context(), userID)
	if err != nil {
		abortServiceError(c, err)
		return
	}
	streakDays, rewardAmount := h.habits.RuleConfig()

	response.Success(c, http.StatusOK, "OK", habitSettingsResponse{
		Enabled:      setting.AutoFundEnabled,
		StreakDays:   streakDays,
		RewardAmount: rewardAmount,
	})
}

// UpdateSettings godoc
// @Summary      Aktifkan/matikan auto-nabung
// @Description  Toggle fitur reward streak (mis. 3 hari tanpa jajan → Rp10.000 otomatis masuk target tabungan aktif tertua).
// @Tags         Habits
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body updateHabitSettingsRequest true "Status toggle"
// @Success      200 {object} response.SuccessBody{data=habitSettingsResponse}
// @Failure      401 {object} response.ErrorBody
// @Router       /habits/settings [put]
func (h *HabitHandler) UpdateSettings(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	var req updateHabitSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	if _, err := h.habits.UpdateSettings(c.Request.Context(), userID, req.Enabled); err != nil {
		abortServiceError(c, err)
		return
	}
	streakDays, rewardAmount := h.habits.RuleConfig()

	response.Success(c, http.StatusOK, "Auto-nabung diperbarui", habitSettingsResponse{
		Enabled:      req.Enabled,
		StreakDays:   streakDays,
		RewardAmount: rewardAmount,
	})
}

// ListLogs godoc
// @Summary      Riwayat reward habit engine
// @Tags         Habits
// @Security     BearerAuth
// @Produce      json
// @Param        limit query int false "Jumlah maksimal data (default 20, max 100)" default(20)
// @Success      200 {object} response.SuccessBody{data=[]model.HabitLog}
// @Failure      401 {object} response.ErrorBody
// @Router       /habits/logs [get]
func (h *HabitHandler) ListLogs(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	limit := 20
	if v := c.Query("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 || parsed > 100 {
			response.Error(c, http.StatusBadRequest, "limit harus angka 1-100", response.ErrCodeValidation)
			return
		}
		limit = parsed
	}

	logs, err := h.habits.ListLogs(c.Request.Context(), userID, limit)
	if err != nil {
		abortServiceError(c, err)
		return
	}
	if logs == nil {
		logs = []model.HabitLog{}
	}
	response.Success(c, http.StatusOK, "OK", logs)
}
