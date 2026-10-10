package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"caupocket-api/internal/middleware"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

// mapServiceError memetakan sentinel error service ke (HTTP status, error_code).
func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return http.StatusUnauthorized, response.ErrCodeInvalidCredentials
	case errors.Is(err, service.ErrEmailTaken):
		return http.StatusConflict, response.ErrCodeEmailTaken
	case errors.Is(err, service.ErrInsufficientBalance):
		return http.StatusUnprocessableEntity, response.ErrCodeInsufficientBalance
	case errors.Is(err, service.ErrGoalAlreadyAchieved):
		return http.StatusConflict, response.ErrCodeGoalAchieved
	case errors.Is(err, service.ErrWalletNotFound):
		return http.StatusNotFound, response.ErrCodeWalletNotFound
	case errors.Is(err, service.ErrGoalNotFound):
		return http.StatusNotFound, response.ErrCodeGoalNotFound
	case errors.Is(err, service.ErrCategoryNotFound):
		return http.StatusNotFound, response.ErrCodeCategoryNotFound
	case errors.Is(err, service.ErrCategoryMismatch):
		return http.StatusBadRequest, response.ErrCodeCategoryMismatch
	case errors.Is(err, service.ErrInvalidAmount), errors.Is(err, service.ErrInvalidInput):
		return http.StatusBadRequest, response.ErrCodeValidation
	default:
		return http.StatusInternalServerError, response.ErrCodeInternal
	}
}

// abortServiceError mengirim error service via Standard Response Wrapper.
func abortServiceError(c *gin.Context, err error) {
	status, code := mapServiceError(err)
	response.Error(c, status, err.Error(), code)
}

// parseFlexibleTime menerima RFC3339 maupun format tanggal "2006-01-02".
func parseFlexibleTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("format tanggal tidak valid, gunakan RFC3339 atau YYYY-MM-DD")
}

// requireUserID mengambil user_id dari context AuthMiddleware; bila tidak ada
// ia sudah mengirim response 401 sehingga handler cukup return.
func requireUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
	}
	return userID, ok
}
