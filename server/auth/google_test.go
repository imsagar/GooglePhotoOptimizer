package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMiddleware_NoSession(t *testing.T) {
	sessionStore := sessions.NewCookieStore([]byte("test-secret"))
	r := gin.New()
	r.GET("/protected", Middleware(sessionStore), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no session cookie, got %d", w.Code)
	}
}

func TestMiddleware_ValidSession(t *testing.T) {
	sessionStore := sessions.NewCookieStore([]byte("test-secret"))
	wantID := uuid.New()

	r := gin.New()
	r.GET("/protected", Middleware(sessionStore), func(c *gin.Context) {
		got, err := UserID(c)
		if err != nil {
			t.Errorf("UserID() error: %v", err)
		}
		if got != wantID {
			t.Errorf("UserID() = %v, want %v", got, wantID)
		}
		c.Status(http.StatusOK)
	})

	// Mint a real session cookie the same way the callback handler would.
	mint := httptest.NewRequest(http.MethodGet, "/", nil)
	mintW := httptest.NewRecorder()
	sess, _ := sessionStore.Get(mint, sessionName)
	sess.Values["user_id"] = wantID.String()
	if err := sess.Save(mint, mintW); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	for _, c := range mintW.Result().Cookies() {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid session, got %d", w.Code)
	}
}
