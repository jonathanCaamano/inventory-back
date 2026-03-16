package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSlidingWindow_AllowsUpToMax(t *testing.T) {
	sw := &slidingWindow{maxHits: 3, windowDur: time.Minute}

	for i := 0; i < 3; i++ {
		if !sw.allow() {
			t.Errorf("attempt %d should be allowed", i+1)
		}
	}
	if sw.allow() {
		t.Error("4th attempt should be rejected")
	}
}

func TestSlidingWindow_ResetsAfterWindow(t *testing.T) {
	sw := &slidingWindow{maxHits: 2, windowDur: 50 * time.Millisecond}

	sw.allow()
	sw.allow()
	if sw.allow() {
		t.Error("3rd attempt within window should be rejected")
	}

	time.Sleep(60 * time.Millisecond)
	if !sw.allow() {
		t.Error("attempt after window expiry should be allowed")
	}
}

func TestLoginRateLimiter_Allows(t *testing.T) {
	r := gin.New()
	r.POST("/login", LoginRateLimiter(5, time.Minute), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("attempt %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestLoginRateLimiter_Blocks(t *testing.T) {
	r := gin.New()
	r.POST("/login", LoginRateLimiter(3, time.Minute), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Exhaust the limit
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		r.ServeHTTP(w, req)
	}

	// Next one should be blocked
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.1:1111"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestLoginRateLimiter_DifferentIPsIndependent(t *testing.T) {
	r := gin.New()
	r.POST("/login", LoginRateLimiter(2, time.Minute), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Exhaust IP1
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		r.ServeHTTP(w, req)
	}

	// IP2 should still be allowed
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.168.1.2:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("different IP should still be allowed, got %d", w.Code)
	}
}

func TestIPLimiter_NewEntryCreated(t *testing.T) {
	l := newIPLimiter(5, time.Minute)
	sw1 := l.get("1.1.1.1")
	sw2 := l.get("1.1.1.1") // same IP
	sw3 := l.get("2.2.2.2") // different IP

	if sw1 != sw2 {
		t.Error("same IP should return same sliding window")
	}
	if sw1 == sw3 {
		t.Error("different IPs should return different sliding windows")
	}
}
