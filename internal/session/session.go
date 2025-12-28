package session

import (
	"RIP/internal/models"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
)

var (
	secret = []byte("VonRodinus005_SuperSecret_2025")
	rdb    = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx    = context.Background()
)

type UserSession struct {
	UserID      uint
	IsModerator bool
}

type Claims struct {
	UserID      uint `json:"user_id"`
	IsModerator bool `json:"is_moderator"`
	jwt.StandardClaims
}

func PingRedis() error {
	_, err := rdb.Ping(ctx).Result()
	return err
}

func CreateSession(user *models.User) string {
	claims := Claims{
		UserID:      user.ID,
		IsModerator: user.IsModerator,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString(secret)
	return s
}

func GetUser(r *http.Request) *UserSession {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil
	}
	t := strings.TrimPrefix(h, "Bearer ")

	if ok, _ := rdb.Get(ctx, "bl:"+t).Result(); ok == "1" {
		return nil
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(t, claims, func(*jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		return nil
	}
	return &UserSession{UserID: claims.UserID, IsModerator: claims.IsModerator}
}

func Logout(w http.ResponseWriter, r *http.Request) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		http.Error(w, "no token", 400)
		return
	}
	t := strings.TrimPrefix(h, "Bearer ")

	claims := &Claims{}
	jwt.ParseWithClaims(t, claims, func(*jwt.Token) (interface{}, error) { return secret, nil })
	ttl := time.Until(time.Unix(claims.ExpiresAt, 0))
	if ttl < 0 {
		ttl = time.Second
	}

	rdb.Set(ctx, "bl:"+t, "1", ttl)
	w.Write([]byte(`{"msg":"logged out"}`))
}
