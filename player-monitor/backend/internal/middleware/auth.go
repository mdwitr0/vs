package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func AuthMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing authorization header"})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(401).JSON(fiber.Map{"error": "invalid authorization header format"})
		}

		tokenString := parts[1]
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(401, "invalid signing method")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := GetRole(c)
		if role != "admin" {
			return c.Status(403).JSON(fiber.Map{"error": "admin access required"})
		}
		return c.Next()
	}
}

func GetUserID(c *fiber.Ctx) string {
	if id, ok := c.Locals("user_id").(string); ok {
		return id
	}
	return ""
}

func GetRole(c *fiber.Ctx) string {
	if role, ok := c.Locals("role").(string); ok {
		return role
	}
	return ""
}

func IsAdmin(c *fiber.Ctx) bool {
	return GetRole(c) == "admin"
}

func GenerateAccessToken(userID, role, secret string, expiry time.Duration) (string, error) {
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
