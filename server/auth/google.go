// Package auth implements Google Sign-In via OAuth2 and a cookie-session
// middleware for the rest of the API.
package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/user/gpoptimizer/server/store"
)

const sessionName = "gpoptimizer_session"

// sharedSessionStore backs the package-level Middleware() so downstream
// packages (Tasks 5/6) can do router.Use(auth.Middleware()) without holding
// a reference to the store built in Routes(). Set once, at startup, by
// Routes(); read-only after that.
var sharedSessionStore *sessions.CookieStore

// Config holds Google OAuth2 credentials and the session signing secret.
// All fields are expected to come from environment variables.
type Config struct {
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	SessionSecret string
}

// ConfigFromEnv reads Config from GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET,
// GOOGLE_REDIRECT_URL and SESSION_SECRET.
func ConfigFromEnv() Config {
	return Config{
		ClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:   os.Getenv("GOOGLE_REDIRECT_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
	}
}

func oauthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func newSessionStore(cfg Config) *sessions.CookieStore {
	s := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	s.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	}
	return s
}

type googleUserInfo struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Routes registers the Google Sign-In endpoints on r:
//
//	GET  /api/auth/google          redirect to Google's consent screen
//	GET  /api/auth/google/callback exchange code, upsert user, start session
//	POST /api/auth/logout          clear session
//	GET  /api/auth/me              current user as JSON
func Routes(r *gin.Engine, db *store.DB, cfg Config) {
	oauthCfg := oauthConfig(cfg)
	sessionStore := newSessionStore(cfg)
	sharedSessionStore = sessionStore

	g := r.Group("/api/auth")
	g.GET("/google", handleGoogleLogin(oauthCfg, sessionStore))
	g.GET("/google/callback", handleCallback(db, oauthCfg, sessionStore))
	g.POST("/logout", handleLogout(sessionStore))
	g.GET("/me", Middleware(), handleMe(sessionStore))
}

func handleGoogleLogin(oauthCfg *oauth2.Config, sessionStore *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := uuid.NewString()
		sess, _ := sessionStore.Get(c.Request, sessionName)
		sess.Values["oauth_state"] = state
		if err := sess.Save(c.Request, c.Writer); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Redirect(http.StatusFound, oauthCfg.AuthCodeURL(state))
	}
}

func handleCallback(db *store.DB, oauthCfg *oauth2.Config, sessionStore *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, _ := sessionStore.Get(c.Request, sessionName)
		wantState, _ := sess.Values["oauth_state"].(string)
		if wantState == "" || c.Query("state") != wantState {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		code := c.Query("code")
		token, err := oauthCfg.Exchange(c.Request.Context(), code)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		client := oauthCfg.Client(c.Request.Context(), token)
		resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err != nil {
			c.AbortWithStatus(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		var info googleUserInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.Email == "" {
			c.AbortWithStatus(http.StatusBadGateway)
			return
		}

		user, err := db.CreateUser(c.Request.Context(), info.Email, info.Name, info.Picture)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		delete(sess.Values, "oauth_state")
		sess.Values["user_id"] = user.ID.String()
		sess.Values["email"] = user.Email
		sess.Values["name"] = user.Name
		sess.Values["picture"] = user.AvatarURL
		if err := sess.Save(c.Request, c.Writer); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Redirect(http.StatusFound, "/")
	}
}

func handleLogout(sessionStore *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, _ := sessionStore.Get(c.Request, sessionName)
		sess.Options.MaxAge = -1
		_ = sess.Save(c.Request, c.Writer)
		c.Status(http.StatusNoContent)
	}
}

// meUser is the trimmed shape returned by /api/auth/me — no need to expose
// the full store.User.
type meUser struct {
	ID      uuid.UUID `json:"id"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	Picture string    `json:"picture"`
}

func handleMe(sessionStore *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("userID").(uuid.UUID)
		sess, _ := sessionStore.Get(c.Request, sessionName)
		email, _ := sess.Values["email"].(string)
		name, _ := sess.Values["name"].(string)
		picture, _ := sess.Values["picture"].(string)
		c.JSON(http.StatusOK, meUser{ID: userID, Email: email, Name: name, Picture: picture})
	}
}

// Middleware reads the session cookie, resolves the logged-in user ID and
// sets it on the context as c.Set("userID", uuid.UUID). Responds 401 if
// there is no valid session. Routes() must be called once at startup before
// any handler using this middleware runs.
func Middleware() gin.HandlerFunc {
	return middlewareFor(sharedSessionStore)
}

func middlewareFor(sessionStore *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, err := sessionStore.Get(c.Request, sessionName)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		raw, _ := sess.Values["user_id"].(string)
		if raw == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		userID, err := uuid.Parse(raw)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("userID", userID)
		c.Next()
	}
}

var errNoUserID = errors.New("no userID in context")

// UserID extracts the authenticated user's ID from the Gin context. It
// panics-free-returns an error if Middleware was not run.
func UserID(c *gin.Context) (uuid.UUID, error) {
	v, ok := c.Get("userID")
	if !ok {
		return uuid.UUID{}, errNoUserID
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return uuid.UUID{}, errNoUserID
	}
	return id, nil
}
