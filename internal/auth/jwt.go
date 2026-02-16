package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID         uuid.UUID `json:"user_id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	jwt.RegisteredClaims
}

// GenerateJWT создаёт JWT с заданным временем жизни.
// expiration — время жизни токена (например 24h). Если <= 0, используется 24h.
func GenerateJWT(secret string, userID uuid.UUID, telegramUserID int64, expiration time.Duration) (string, error) {
	if expiration <= 0 {
		expiration = 24 * time.Hour
	}

	claims := Claims{
		UserID:         userID,
		TelegramUserID: telegramUserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ads-marketplace",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseJWT(secret string, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
