package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"caupocket-api/internal/model"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

type createGoalRequest struct {
	Title        string `json:"title" binding:"required,min=1,max=255" example:"Liburan ke Bali"`
	TargetAmount int64  `json:"target_amount" binding:"required,gt=0" example:"5000000"`
	TargetDate   string `json:"target_date" example:"2026-12-20"`
}

// GoalHandler HTTP handler pengelolaan target tabungan.
type GoalHandler struct {
	goals service.GoalService
}

func NewGoalHandler(goals service.GoalService) *GoalHandler {
	return &GoalHandler{goals: goals}
}

// CreateGoal godoc
// @Summary      Buat target tabungan baru
// @Tags         Goals
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body createGoalRequest true "Data target tabungan"
// @Success      201 {object} response.SuccessBody{data=model.SavingGoal}
// @Failure      400 {object} response.ErrorBody
// @Failure      401 {object} response.ErrorBody
// @Router       /goals [post]
func (h *GoalHandler) CreateGoal(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	var req createGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	var targetDate *time.Time
	if req.TargetDate != "" {
		parsed, err := parseFlexibleTime(req.TargetDate)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
			return
		}
		targetDate = &parsed
	}

	goal, err := h.goals.Create(c.Request.Context(), userID, service.CreateGoalInput{
		Title:        req.Title,
		TargetAmount: req.TargetAmount,
		TargetDate:   targetDate,
	})
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "Target tabungan berhasil dibuat", goal)
}

// ListGoals godoc
// @Summary      Daftar seluruh target tabungan
// @Description  Goal berstatus ACTIVE tampil lebih dulu, lalu ACHIEVED.
// @Tags         Goals
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} response.SuccessBody{data=[]model.SavingGoal}
// @Failure      401 {object} response.ErrorBody
// @Router       /goals [get]
func (h *GoalHandler) ListGoals(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	goals, err := h.goals.ListByUser(c.Request.Context(), userID)
	if err != nil {
		abortServiceError(c, err)
		return
	}
	if goals == nil {
		goals = []model.SavingGoal{}
	}
	response.Success(c, http.StatusOK, "OK", goals)
}
