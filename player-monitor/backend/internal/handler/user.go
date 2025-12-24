package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/player-monitor/backend/internal/repo"
	"golang.org/x/crypto/bcrypt"
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

func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	user, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch user"})
	}
	if user == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "user not found"})
	}

	if err := h.userRepo.Delete(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to delete user"})
	}

	return c.JSON(SuccessResponse{Message: "user deleted"})
}

func (h *UserHandler) UpdateStatus(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	user, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch user"})
	}
	if user == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "user not found"})
	}

	if err := h.userRepo.UpdateStatus(c.Context(), id, req.IsActive); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to update user status"})
	}

	return c.JSON(SuccessResponse{Message: "user status updated"})
}
