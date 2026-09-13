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

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/server/relay"
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
var photosScopes = []string{
	"https://www.googleapis.com/auth/photospicker.mediaitems.readonly",
	"https://www.googleapis.com/auth/drive.file",
}

func Routes(r *gin.Engine, db *store.DB, cfg Config, rl relay.Relay) {
	oauthCfg := oauthConfig(cfg)
	sessionStore := newSessionStore(cfg)
	sharedSessionStore = sessionStore

	g := r.Group("/api/auth")
	g.GET("/google", handleGoogleLogin(oauthCfg, sessionStore))
	g.GET("/google/callback", handleCallback(db, oauthCfg, sessionStore, cfg, rl))
	g.POST("/logout", handleLogout(sessionStore))
	g.GET("/me", Middleware(), handleMe(sessionStore))
	g.GET("/google/photos", Middleware(), handleGooglePhotosLogin(cfg, sessionStore))
	g.GET("/google/photos/status", Middleware(), handleGooglePhotosStatus(db))
	g.POST("/google/photos/disconnect", Middleware(), handleGooglePhotosDisconnect(db))
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

func handleGooglePhotosLogin(cfg Config, sessionStore *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		photosCfg := &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       photosScopes,
			Endpoint:     google.Endpoint,
		}
		state := uuid.NewString()
		sess, _ := sessionStore.Get(c.Request, sessionName)
		sess.Values["oauth_state"] = state
		sess.Values["oauth_flow"] = "photos"
		if err := sess.Save(c.Request, c.Writer); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Redirect(http.StatusFound, photosCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent")))
	}
}

func handleCallback(db *store.DB, oauthCfg *oauth2.Config, sessionStore *sessions.CookieStore, cfg Config, rl relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, _ := sessionStore.Get(c.Request, sessionName)
		wantState, _ := sess.Values["oauth_state"].(string)
		if wantState == "" || c.Query("state") != wantState {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		flow, _ := sess.Values["oauth_flow"].(string)
		delete(sess.Values, "oauth_flow")

		code := c.Query("code")

		if flow == "photos" {
			raw, _ := sess.Values["user_id"].(string)
			userID, err := uuid.Parse(raw)
			if err != nil || raw == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			photosCfg := &oauth2.Config{
				ClientID:     oauthCfg.ClientID,
				ClientSecret: oauthCfg.ClientSecret,
				RedirectURL:  oauthCfg.RedirectURL,
				Scopes:       photosScopes,
				Endpoint:     google.Endpoint,
			}
			token, err := photosCfg.Exchange(c.Request.Context(), code)
			if err != nil {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			tokenJSON, _ := json.Marshal(token)
			if err := db.SaveGoogleToken(c.Request.Context(), userID, string(tokenJSON)); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			credCmd, _ := json.Marshal(protocol.Command{
				Type:         "google_credentials",
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				TokenJSON:    string(tokenJSON),
			})
			rl.SendToRunner(userID, credCmd)
			delete(sess.Values, "oauth_state")
			_ = sess.Save(c.Request, c.Writer)
			c.Redirect(http.StatusFound, "/settings")
			return
		}

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

func handleGooglePhotosStatus(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("userID").(uuid.UUID)
		tok, _ := db.GetGoogleToken(c.Request.Context(), userID)
		c.JSON(http.StatusOK, gin.H{"connected": tok != ""})
	}
}

func handleGooglePhotosDisconnect(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("userID").(uuid.UUID)
		if err := db.DeleteGoogleToken(c.Request.Context(), userID); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
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
