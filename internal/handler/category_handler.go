package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"caupocket-api/internal/middleware"
	"caupocket-api/internal/model"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

type createCategoryRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100" example:"Kopi"`
	Type     string `json:"type" binding:"required,oneof=INCOME EXPENSE" example:"EXPENSE"`
	IconName string `json:"icon_name" example:"coffee"`
}

// CategoryHandler HTTP handler kategori.
type CategoryHandler struct {
	categories service.CategoryService
}

func NewCategoryHandler(categories service.CategoryService) *CategoryHandler {
	return &CategoryHandler{categories: categories}
}

// ListCategories godoc
// @Summary      Daftar kategori milik user
// @Tags         Categories
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} response.SuccessBody{data=[]model.Category}
// @Failure      401 {object} response.ErrorBody
// @Router       /categories [get]
func (h *CategoryHandler) ListCategories(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	categories, err := h.categories.ListByUser(c.Request.Context(), userID)
	if err != nil {
		abortServiceError(c, err)
		return
	}
	if categories == nil {
		categories = []model.Category{}
	}
	response.Success(c, http.StatusOK, "OK", categories)
}

// CreateCategory godoc
// @Summary      Buat kategori baru
// @Tags         Categories
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body createCategoryRequest true "Data kategori"
// @Success      201 {object} response.SuccessBody{data=model.Category}
// @Failure      400 {object} response.ErrorBody
// @Failure      401 {object} response.ErrorBody
// @Router       /categories [post]
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	if req.IconName == "" {
		req.IconName = "tag"
	}

	category, err := h.categories.Create(c.Request.Context(), userID, service.CreateCategoryInput{
		Name:     req.Name,
		Type:     model.CategoryType(req.Type),
		IconName: req.IconName,
	})
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "Kategori berhasil dibuat", category)
}
