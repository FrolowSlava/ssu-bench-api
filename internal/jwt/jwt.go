package jwt

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"ssu-bench-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int         `json:"user_id"`
	Email  string      `json:"email"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
	jwtExpiresIn  time.Duration
	jwtOnce       sync.Once
)

// initJWT загружает настройки JWT при первом вызове
func initJWT() {
	jwtOnce.Do(func() {
		// Загрузка секрета
		secret := os.Getenv("JWT_SECRET")
		isProd := strings.ToLower(os.Getenv("GIN_MODE")) == "release"

		if secret == "" {
			if isProd {
				panic("[FATAL] JWT_SECRET is required in production mode")
			}
			fmt.Println("[WARN] JWT_SECRET not set! Using dev fallback.")
			fmt.Println("[WARN] Set JWT_SECRET in .env for production!")
			secret = "dev-secret-do-not-use-in-production"
		}

		if len(secret) < 32 && !isProd {
			fmt.Printf("[WARN] JWT_SECRET is %d bytes (recommended: 32+)\n", len(secret))
		}

		jwtSecret = []byte(secret)

		// Загрузка времени жизни токена
		expiresIn := os.Getenv("JWT_EXPIRES_IN")
		if expiresIn == "" {
			expiresIn = "24h" // значение по умолчанию
		}

		duration, err := time.ParseDuration(expiresIn)
		if err != nil {
			fmt.Printf("[WARN] Invalid JWT_EXPIRES_IN '%s', using default 24h\n", expiresIn)
			duration = 24 * time.Hour
		}
		jwtExpiresIn = duration
	})
}

// getJWTSecret загружает секрет при первом вызове
func getJWTSecret() []byte {
	initJWT()
	return jwtSecret
}

// getJWTExpiresIn возвращает время жизни токена
func getJWTExpiresIn() time.Duration {
	initJWT()
	return jwtExpiresIn
}

func GenerateToken(user *models.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(getJWTExpiresIn())),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "ssu-bench-api",
			Subject:   fmt.Sprintf("user:%d", user.ID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return getJWTSecret(), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, errors.New("invalid token format")
		}
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token claims")
}
