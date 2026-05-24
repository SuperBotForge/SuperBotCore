package jwt

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/do/v2"
	"time"
)

type Config struct {
	Secret []byte
}

type Coder struct {
	config Config
}

func NewCoder(i do.Injector) (*Coder, error) {
	return &Coder{
		config: do.MustInvoke[Config](i),
	}, nil
}

type Claims struct {
	CourseId int64 `json:"courseId"`
	jwt.RegisteredClaims
}

func (s *Coder) CreateJwtTokenWithCourseId(courseId int64) (string, error) {
	expirationTime := time.Now().Add(24 * 30 * time.Hour)
	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		CourseId: courseId,
		RegisteredClaims: jwt.RegisteredClaims{
			// In JWT, the expiry time is expressed as unix milliseconds
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(s.config.Secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *Coder) GetCourseId(tokenString string) (int64, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return s.config.Secret, nil
	})

	if err != nil {
		return -1, err
	}

	if !token.Valid {
		return -1, fmt.Errorf("invalid token")
	}

	return claims.CourseId, nil
}
