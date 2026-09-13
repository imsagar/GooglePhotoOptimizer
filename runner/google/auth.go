// Package google implements the runner's Google Photos + Drive API clients,
// authenticated via a local-machine OAuth flow (browser + localhost callback).
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/oauth2"
	oauthgoogle "golang.org/x/oauth2/google"

	"github.com/user/gpoptimizer/runner/config"
)

var Scopes = []string{
	"https://www.googleapis.com/auth/photospicker.mediaitems.readonly",
	"https://www.googleapis.com/auth/drive.file",
}

// NewOAuthConfig builds the oauth2.Config shared by StartLocalAuth and the
// Photos/Drive clients (they need the same config to refresh tokens).
func NewOAuthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       Scopes,
		Endpoint:     oauthgoogle.Endpoint,
		RedirectURL:  "http://localhost:9876/callback",
	}
}

// StartLocalAuth runs the local OAuth flow: opens the user's browser, listens
// on localhost for the redirect, and exchanges the code for a token. Returns
// both the token and the config used to obtain it, since refreshing the token
// later (and constructing Photos/Drive clients) needs the same config.
func StartLocalAuth(clientID, clientSecret string) (*oauth2.Token, *oauth2.Config, error) {
	cfg := NewOAuthConfig(clientID, clientSecret)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback")
			return
		}
		fmt.Fprintf(w, "<h1>Authorized!</h1><p>You can close this tab.</p>")
		codeCh <- code
	})

	ln, err := net.Listen("tcp", "localhost:9876")
	if err != nil {
		return nil, nil, err
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	authURL := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
	openBrowser(authURL)
	fmt.Printf("If browser didn't open, visit:\n%s\n", authURL)

	select {
	case code := <-codeCh:
		tok, err := cfg.Exchange(context.Background(), code)
		if err != nil {
			return nil, nil, err
		}
		return tok, cfg, nil
	case err := <-errCh:
		return nil, nil, err
	}
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

// tokenPath returns ~/.gpoptimizer/google_token.json.
func tokenPath() string {
	return filepath.Join(config.Dir(), "google_token.json")
}

// SaveToken persists the OAuth token to disk so the user doesn't have to
// re-authenticate on every run.
func SaveToken(tok *oauth2.Token) error {
	if err := os.MkdirAll(config.Dir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenPath(), data, 0600)
}

// LoadToken reads a previously saved OAuth token from disk.
// ponytail: in-memory refresh works but refreshed token isn't persisted back;
// wrap with oauth2.ReuseTokenSource + save-on-refresh callback in Task 10's main loop
func LoadToken() (*oauth2.Token, error) {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}
