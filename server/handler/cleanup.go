package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/relay"
)

// HandleListLocalFiles asks the runner to report the files it has sitting on
// local disk (originals/optimized outputs). Fire-and-forget: the runner's
// answer arrives asynchronously as a status message over /ws/runner, which
// ws.go forwards to the user's UI socket as-is (a "local_files" status
// carrying protocol.Status.Files).
func HandleListLocalFiles(r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		cmd, err := json.Marshal(protocol.Command{Type: "list_local_files"})
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		r.SendToRunner(userID, cmd)
		c.JSON(http.StatusAccepted, gin.H{"status": "requested"})
	}
}

type deleteLocalFilesRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// HandleDeleteLocalFiles tells the runner to delete the given local file
// paths (e.g. originals the user has confirmed are safe to remove).
func HandleDeleteLocalFiles(r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		var req deleteLocalFilesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cmd, err := json.Marshal(protocol.Command{Type: "delete_local", Targets: req.Paths})
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		r.SendToRunner(userID, cmd)
		c.JSON(http.StatusAccepted, gin.H{"status": "requested"})
	}
}
