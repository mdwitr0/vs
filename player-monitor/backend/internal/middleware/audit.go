package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/player-monitor/backend/internal/repo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditMiddleware struct {
	auditRepo *repo.AuditLogRepo
}

func NewAuditMiddleware(auditRepo *repo.AuditLogRepo) *AuditMiddleware {
	return &AuditMiddleware{auditRepo: auditRepo}
}

func (m *AuditMiddleware) Log(action string, details map[string]any) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return c.Next()
		}

		userOID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Next()
		}

		ip := c.IP()

		auditLog := &repo.AuditLog{
			UserID:    userOID,
			Action:    action,
			Details:   details,
			IPAddress: ip,
		}

		go m.auditRepo.Create(c.Context(), auditLog)

		return c.Next()
	}
}
