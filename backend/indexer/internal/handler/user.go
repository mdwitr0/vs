package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/video-analitics/indexer/internal/repo"
)

type UserHandler struct {
	userRepo *repo.UserRepo
}

func NewUserHandler(userRepo *repo.UserRepo) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

type CreateUserRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateUserRequest struct {
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role,omitempty"`
	IsActive *bool  `json:"is_active,omitempty"`
}

type UsersListResponse struct {
	Items []repo.User `json:"items"`
	Total int64       `json:"total"`
}

// List godoc
// @Summary List users (admin only)
// @Description Get list of all users with pagination
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param role query string false "Filter by role (admin, user)"
// @Param is_active query bool false "Filter by active status"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} UsersListResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/users [get]
func (h *UserHandler) List(c *fiber.Ctx) error {
	role := c.Query("role")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	filter := repo.UserFilter{
		Role:   role,
		Limit:  limit,
		Offset: offset,
	}

	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		filter.IsActive = &isActive
	}

	users, total, err := h.userRepo.FindAll(c.Context(), filter)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch users"})
	}

	return c.JSON(UsersListResponse{
		Items: users,
		Total: total,
	})
}

// Create godoc
// @Summary Create user (admin only)
// @Description Create a new user
// @Tags users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body CreateUserRequest true "User data"
// @Success 201 {object} repo.User
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/users [post]
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.Login == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "login is required"})
	}
	if req.Password == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "password is required"})
	}
	if req.Role != "" && req.Role != "admin" && req.Role != "user" {
		return c.Status(400).JSON(ErrorResponse{Error: "role must be 'admin' or 'user'"})
	}

	existing, _ := h.userRepo.FindByLogin(c.Context(), req.Login)
	if existing != nil {
		return c.Status(409).JSON(ErrorResponse{Error: "user with this login already exists"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to hash password"})
	}

	user := &repo.User{
		Login:        req.Login,
		PasswordHash: string(hash),
		Role:         req.Role,
	}

	if err := h.userRepo.Create(c.Context(), user); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create user"})
	}

	return c.Status(201).JSON(user)
}

// Update godoc
// @Summary Update user (admin only)
// @Description Update user details
// @Tags users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body UpdateUserRequest true "User data"
// @Success 200 {object} repo.User
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/users/{id} [put]
func (h *UserHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	user, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch user"})
	}
	if user == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "user not found"})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.Login != "" {
		if req.Login != user.Login {
			existing, _ := h.userRepo.FindByLogin(c.Context(), req.Login)
			if existing != nil {
				return c.Status(409).JSON(ErrorResponse{Error: "login already in use"})
			}
		}
		user.Login = req.Login
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{Error: "failed to hash password"})
		}
		user.PasswordHash = string(hash)
	}

	if req.Role != "" {
		if req.Role != "admin" && req.Role != "user" {
			return c.Status(400).JSON(ErrorResponse{Error: "role must be 'admin' or 'user'"})
		}
		user.Role = req.Role
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := h.userRepo.Update(c.Context(), user); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to update user"})
	}

	return c.JSON(user)
}

// Delete godoc
// @Summary Soft delete user (admin only)
// @Description Deactivate a user (soft delete)
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} SuccessResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/users/{id} [delete]
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	user, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch user"})
	}
	if user == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "user not found"})
	}

	if err := h.userRepo.SoftDelete(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to delete user"})
	}

	return c.JSON(SuccessResponse{Message: "user deactivated"})
}
