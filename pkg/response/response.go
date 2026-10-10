// Package response mendefinisikan Standard Response Wrapper JSON untuk seluruh endpoint.
package response

import "github.com/gin-gonic/gin"

// Kode error yang dikonsumsi frontend untuk penanganan error yang spesifik.
const (
	ErrCodeValidation          = "VALIDATION_ERROR"
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeInvalidCredentials  = "INVALID_CREDENTIALS"
	ErrCodeEmailTaken          = "EMAIL_ALREADY_EXISTS"
	ErrCodeInsufficientBalance = "INSUFFICIENT_BALANCE"
	ErrCodeWalletNotFound      = "WALLET_NOT_FOUND"
	ErrCodeGoalNotFound        = "GOAL_NOT_FOUND"
	ErrCodeGoalAchieved        = "GOAL_ALREADY_ACHIEVED"
	ErrCodeCategoryNotFound    = "CATEGORY_NOT_FOUND"
	ErrCodeCategoryMismatch    = "CATEGORY_TYPE_MISMATCH"
	ErrCodeNotFound            = "NOT_FOUND"
	ErrCodeInternal            = "INTERNAL_ERROR"
)

// SuccessBody format envelope sukses (2xx).
type SuccessBody struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Operasi berhasil"`
	Data    any    `json:"data"`
}

// ErrorBody format envelope error (4xx/5xx).
type ErrorBody struct {
	Success   bool   `json:"success" example:"false"`
	Message   string `json:"message" example:"Terjadi kesalahan"`
	ErrorCode string `json:"error_code" example:"INTERNAL_ERROR"`
}

// Success mengirim response sukses dengan Standard Response Wrapper.
func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, SuccessBody{Success: true, Message: message, Data: data})
}

// Error mengirim response error dengan Standard Response Wrapper.
func Error(c *gin.Context, status int, message string, errorCode string) {
	c.JSON(status, ErrorBody{Success: false, Message: message, ErrorCode: errorCode})
}
