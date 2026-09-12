// Package pairing exchanges a short-lived pairing code (shown in the web UI)
// for a runner bearer token, per POST /api/runner/register.
package pairing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/user/gpoptimizer/runner/config"
)

type registerResponse struct {
	RunnerID string `json:"runner_id"`
	Token    string `json:"token"`
}

// Pair registers this machine as a runner with the server using the pairing
// code from the web UI. The server generates and returns the bearer token;
// Pair saves it (with serverURL) to the runner's local config.
func Pair(serverURL, code string) (*config.Config, error) {
	body, err := json.Marshal(map[string]string{
		"pairing_code": code,
		"platform":     runtime.GOOS,
		"arch":         runtime.GOARCH,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(serverURL+"/api/runner/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pairing failed (%d): %s", resp.StatusCode, respBody)
	}

	var reg registerResponse
	if err := json.Unmarshal(respBody, &reg); err != nil {
		return nil, fmt.Errorf("pairing response: %w", err)
	}
	if reg.Token == "" {
		return nil, fmt.Errorf("pairing response missing token")
	}

	cfg := &config.Config{
		ServerURL:   serverURL,
		Token:       reg.Token,
		StoragePath: "~/GooglePhotosOptimized",
	}
	if err := config.Save(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
