package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/store"
)

// HandlePair generates a short-lived pairing code the user types into the
// local runner during setup.
func HandlePair(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		pc, err := db.CreatePairingCode(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": pc.Code, "expires_at": pc.ExpiresAt})
	}
}

// newRunnerToken returns a random 32-byte token, hex-encoded, for the runner
// to use as its bearer credential over /ws/runner.
func newRunnerToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type registerRequest struct {
	PairingCode string `json:"pairing_code" binding:"required"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
}

// HandleRegister is public (no session) — the runner has no auth of its own
// yet. It exchanges a pairing code for a fresh bearer token and creates the
// runner row storing only the bcrypt hash of that token.
func HandleRegister(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		pc, err := db.ValidatePairingCode(c.Request.Context(), req.PairingCode)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired pairing code"})
			return
		}

		token, err := newRunnerToken()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		runner, err := db.CreateRunner(c.Request.Context(), pc.UserID, string(hash), req.Platform, req.Arch)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{"runner_id": runner.ID, "token": token})
	}
}

// HandleRotate issues a new bearer token for the caller's runner, invalidating
// the old one. The caller is authenticated as a user (session), not as the
// runner itself — this is meant for "revoke and re-pair" from the UI.
func HandleRotate(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		runner, err := db.GetRunnerByUserID(c.Request.Context(), userID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "no runner registered"})
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		token, err := newRunnerToken()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if err := db.UpdateRunnerTokenHash(c.Request.Context(), runner.ID, string(hash)); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{"runner_id": runner.ID, "token": token})
	}
}
