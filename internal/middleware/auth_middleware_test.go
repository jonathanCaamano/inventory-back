package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testSecret = "test-jwt-secret-key-32-chars-long!!"

func makeToken(t *testing.T, userID uuid.UUID, role models.Role, secret string, ttl time.Duration) string {
	t.Helper()
	claims := &services.Claims{
		UserID:   userID,
		Username: "testuser",
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func makeAuthService() *services.AuthService {
	return services.NewAuthService(nil, nil, testSecret, 1)
}

func TestAuthRequired_NoHeader(t *testing.T) {
	r := gin.New()
	r.GET("/", AuthRequired(makeAuthService()), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidFormat(t *testing.T) {
	r := gin.New()
	r.GET("/", AuthRequired(makeAuthService()), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	r := gin.New()
	r.GET("/", AuthRequired(makeAuthService()), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, userID, models.RoleAdmin, testSecret, time.Hour)

	r := gin.New()
	r.GET("/", AuthRequired(makeAuthService()), func(c *gin.Context) {
		id, ok := GetUserID(c)
		if !ok || id != userID {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthRequired_ExpiredToken(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, userID, models.RoleAdmin, testSecret, -time.Hour)

	r := gin.New()
	r.GET("/", AuthRequired(makeAuthService()), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.Set(ContextKeyRole, models.RoleAdmin)
		c.Next()
	}, RequireRole(models.RoleAdmin, models.RoleManager), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.Set(ContextKeyRole, models.RoleViewer)
		c.Next()
	}, RequireRole(models.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireRole_NoRoleInContext(t *testing.T) {
	r := gin.New()
	r.GET("/", RequireRole(models.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetUserID(t *testing.T) {
	userID := uuid.New()
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.Set(ContextKeyUserID, userID)
		id, ok := GetUserID(c)
		if !ok || id != userID {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetUserID_Missing(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		_, ok := GetUserID(c)
		if ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetUserRole(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.Set(ContextKeyRole, models.RoleAdmin)
		role, ok := GetUserRole(c)
		if !ok || role != models.RoleAdmin {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
