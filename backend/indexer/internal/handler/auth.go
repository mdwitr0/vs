package handler

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/video-analitics/indexer/internal/middleware"
	"github.com/video-analitics/indexer/internal/repo"
)

type AuthHandler struct {
	userRepo         *repo.UserRepo
	refreshTokenRepo *repo.RefreshTokenRepo
	jwtSecret        string
	accessExpiry     time.Duration
	refreshExpiry    time.Duration
}

func NewAuthHandler(
	userRepo *repo.UserRepo,
	refreshTokenRepo *repo.RefreshTokenRepo,
	jwtSecret string,
	accessExpiry, refreshExpiry time.Duration,
) *AuthHandler {
	return &AuthHandler{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtSecret:        jwtSecret,
		accessExpiry:     accessExpiry,
		refreshExpiry:    refreshExpiry,
	}
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

// Login godoc
// @Summary Login
// @Description Authenticate user and get tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Credentials"
// @Success 200 {object} TokenResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.Login == "" || req.Password == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "login and password are required"})
	}

	user, err := h.userRepo.FindByLogin(c.Context(), req.Login)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "internal server error"})
	}
	if user == nil {
		return c.Status(401).JSON(ErrorResponse{Error: "invalid credentials"})
	}

	if !user.IsActive {
		return c.Status(401).JSON(ErrorResponse{Error: "account is deactivated"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(ErrorResponse{Error: "invalid credentials"})
	}

	tokens, err := h.generateTokens(c, user)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to generate tokens"})
	}

	return c.JSON(tokens)
}

// Refresh godoc
// @Summary Refresh access token
// @Description Get new access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RefreshRequest true "Refresh token"
// @Success 200 {object} TokenResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.RefreshToken == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "refresh_token is required"})
	}

	tokenBytes, err := hex.DecodeString(req.RefreshToken)
	if err != nil || len(tokenBytes) < 16 {
		return c.Status(401).JSON(ErrorResponse{Error: "invalid refresh token"})
	}

	userID := string(tokenBytes[:24])

	storedToken, err := h.refreshTokenRepo.FindByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "internal server error"})
	}
	if storedToken == nil {
		return c.Status(401).JSON(ErrorResponse{Error: "invalid refresh token"})
	}

	if time.Now().After(storedToken.ExpiresAt) {
		h.refreshTokenRepo.DeleteByUserID(c.Context(), userID)
		return c.Status(401).JSON(ErrorResponse{Error: "refresh token expired"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedToken.TokenHash), tokenBytes); err != nil {
		return c.Status(401).JSON(ErrorResponse{Error: "invalid refresh token"})
	}

	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "internal server error"})
	}
	if user == nil || !user.IsActive {
		return c.Status(401).JSON(ErrorResponse{Error: "user not found or deactivated"})
	}

	tokens, err := h.generateTokens(c, user)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to generate tokens"})
	}

	return c.JSON(tokens)
}

// Logout godoc
// @Summary Logout
// @Description Invalidate refresh token
// @Tags auth
// @Security BearerAuth
// @Success 200 {object} SuccessResponse
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{Error: "unauthorized"})
	}

	h.refreshTokenRepo.DeleteByUserID(c.Context(), userID)

	return c.JSON(SuccessResponse{Message: "logged out successfully"})
}

// Me godoc
// @Summary Get current user
// @Description Get authenticated user profile
// @Tags auth
// @Security BearerAuth
// @Success 200 {object} repo.User
// @Failure 401 {object} ErrorResponse
// @Router /api/auth/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{Error: "unauthorized"})
	}

	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "internal server error"})
	}
	if user == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "user not found"})
	}

	return c.JSON(user)
}

func (h *AuthHandler) generateTokens(c *fiber.Ctx, user *repo.User) (*TokenResponse, error) {
	accessToken, err := middleware.GenerateAccessToken(
		user.ID.Hex(),
		user.Role,
		h.jwtSecret,
		h.accessExpiry,
	)
	if err != nil {
		return nil, err
	}

	refreshTokenBytes := make([]byte, 32)
	copy(refreshTokenBytes[:24], []byte(user.ID.Hex()))
	if _, err := rand.Read(refreshTokenBytes[24:]); err != nil {
		return nil, err
	}

	refreshTokenHash, err := bcrypt.GenerateFromPassword(refreshTokenBytes, bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	refreshToken := &repo.RefreshToken{
		UserID:    user.ID,
		TokenHash: string(refreshTokenHash),
		ExpiresAt: time.Now().Add(h.refreshExpiry),
	}

	if err := h.refreshTokenRepo.Upsert(c.Context(), refreshToken); err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: hex.EncodeToString(refreshTokenBytes),
		ExpiresIn:    int64(h.accessExpiry.Seconds()),
	}, nil
}
