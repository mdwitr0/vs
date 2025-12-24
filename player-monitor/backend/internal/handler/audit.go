package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/player-monitor/backend/internal/middleware"
	"github.com/player-monitor/backend/internal/repo"
)

type AuditHandler struct {
	auditRepo *repo.AuditLogRepo
}

func NewAuditHandler(auditRepo *repo.AuditLogRepo) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo}
}

type AuditLogsListResponse struct {
	Items []repo.AuditLog `json:"items"`
	Total int64           `json:"total"`
}

func (h *AuditHandler) List(c *fiber.Ctx) error {
	currentUserID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	action := c.Query("action")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	filter := repo.AuditLogFilter{
		Action: action,
		Limit:  limit,
		Offset: offset,
	}

	if !isAdmin {
		filter.UserID = currentUserID
	}

	logs, total, err := h.auditRepo.FindAll(c.Context(), filter)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch audit logs"})
	}

	return c.JSON(AuditLogsListResponse{
		Items: logs,
		Total: total,
	})
}
