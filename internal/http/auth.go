package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	IsActive     bool   `json:"isActive"`
	PasswordHash string `json:"-"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type authClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	var user authUser
	err := h.db.Table("users").
		Select("id, username, name, email, role, is_active, password_hash").
		Where("LOWER(username) = LOWER(?)", strings.TrimSpace(req.Username)).
		First(&user).Error
	if err != nil || !user.IsActive || user.Role != "admin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.issueJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}
	refreshToken, err := h.issueRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"token":        token,
		"refreshToken": refreshToken,
		"user": gin.H{"id": user.ID, "username": user.Username, "name": user.Name, "email": user.Email, "role": user.Role},
	}})
}

func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err == nil && strings.TrimSpace(req.RefreshToken) != "" {
		tokenHash := hashToken(req.RefreshToken)
		_ = h.db.Exec("UPDATE admin_refresh_tokens SET revoked_at = NOW() WHERE token_hash = ? AND revoked_at IS NULL", tokenHash).Error
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true}})
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}

	tokenHash := hashToken(req.RefreshToken)
	row := struct {
		ID        int64     `json:"id"`
		UserID    int64     `json:"userId"`
		ExpiresAt time.Time `json:"expiresAt"`
		RevokedAt *time.Time
	}{}
	err := h.db.Table("admin_refresh_tokens").
		Select("id, user_id, expires_at, revoked_at").
		Where("token_hash = ?", tokenHash).
		First(&row).Error
	if err != nil || row.RevokedAt != nil || row.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	var user authUser
	err = h.db.Table("users").
		Select("id, username, name, email, role, is_active").
		Where("id = ?", row.UserID).
		First(&user).Error
	if err != nil || !user.IsActive || user.Role != "admin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.db.Exec("UPDATE admin_refresh_tokens SET revoked_at = NOW() WHERE id = ?", row.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate refresh token"})
		return
	}

	accessToken, err := h.issueJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue access token"})
		return
	}
	refreshToken, err := h.issueRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": accessToken, "refreshToken": refreshToken}})
}

func (h *Handler) Me(c *gin.Context) {
	claims, ok := h.requireJWT(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
		return
	}

	var user authUser
	err = h.db.Table("users").
		Select("id, username, name, email, role, is_active").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil || !user.IsActive || user.Role != "admin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": user.ID, "username": user.Username, "name": user.Name, "email": user.Email, "role": user.Role}})
}

func (h *Handler) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := h.requireJWT(c)
		if !ok {
			return
		}
		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}
		if claims.Role != "admin" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		var user authUser
		err = h.db.Table("users").Select("id, role, is_active").Where("id = ?", userID).First(&user).Error
		if err != nil || !user.IsActive || user.Role != "admin" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Set("adminUserID", user.ID)
		c.Next()
	}
}

func (h *Handler) issueJWT(user authUser) (string, error) {
	now := time.Now()
	claims := authClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

func (h *Handler) issueRefreshToken(userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	if err := h.db.Exec(
		"INSERT INTO admin_refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)",
		userID,
		tokenHash,
		expiresAt,
	).Error; err != nil {
		return "", err
	}

	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func (h *Handler) requireJWT(c *gin.Context) (authClaims, bool) {
	raw := c.GetHeader("Authorization")
	if raw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		c.Abort()
		return authClaims{}, false
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
		c.Abort()
		return authClaims{}, false
	}

	tokenStr := strings.TrimSpace(parts[1])
	parsed, err := jwt.ParseWithClaims(tokenStr, &authClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(h.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		c.Abort()
		return authClaims{}, false
	}

	claims, ok := parsed.Claims.(*authClaims)
	if !ok || claims.Subject == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
		c.Abort()
		return authClaims{}, false
	}

	return *claims, true
}
