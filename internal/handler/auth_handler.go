package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"caupocket-api/internal/middleware"
	"caupocket-api/internal/model"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"
)

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u *model.User) userResponse {
	return userResponse{ID: u.ID.String(), Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt}
}

type registerRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=255" example:"Naufal Aditya"`
	Email    string `json:"email" binding:"required,email" example:"naufal@example.com"`
	Password string `json:"password" binding:"required,min=8,max=72" example:"rahasia123"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"naufal@example.com"`
	Password string `json:"password" binding:"required" example:"rahasia123"`
}

type loginData struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

// AuthHandler HTTP handler untuk autentikasi.
type AuthHandler struct {
	auth service.AuthService
}

func NewAuthHandler(auth service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Register godoc
// @Summary      Daftar akun baru
// @Description  Membuat user beserta dompet utama ("Dompet Utama") dan kategori default dalam satu transaksi.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body registerRequest true "Data pendaftaran"
// @Success      201 {object} response.SuccessBody{data=userResponse}
// @Failure      400 {object} response.ErrorBody
// @Failure      409 {object} response.ErrorBody
// @Failure      500 {object} response.ErrorBody
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	user, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, "Pendaftaran berhasil", toUserResponse(user))
}

// Login godoc
// @Summary      Login
// @Description  Memvalidasi kredensial (bcrypt) lalu mengembalikan JWT yang memuat user_id.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body loginRequest true "Kredensial login"
// @Success      200 {object} response.SuccessBody{data=loginData}
// @Failure      401 {object} response.ErrorBody
// @Failure      500 {object} response.ErrorBody
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), response.ErrCodeValidation)
		return
	}

	token, user, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		abortServiceError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "Login berhasil", loginData{Token: token, User: toUserResponse(user)})
}

// Me godoc
// @Summary      Profil user saat ini
// @Description  Mengembalikan data user pemilik token. Berguna untuk memvalidasi cookie/token dari frontend.
// @Tags         Auth
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} response.SuccessBody{data=userResponse}
// @Failure      401 {object} response.ErrorBody
// @Router       /me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Tidak terautentikasi", response.ErrCodeUnauthorized)
		return
	}

	user, err := h.auth.GetUser(c.Request.Context(), userID)
	if err != nil {
		abortServiceError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "OK", toUserResponse(user))
}
