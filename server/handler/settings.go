package handler

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/user/gpoptimizer/server/auth"
)

// jobSettings are the per-user defaults applied when the frontend creates
// jobs without specifying codec/crf/preset itself.
type jobSettings struct {
	Codec       string `json:"codec"`
	CRF         int    `json:"crf"`
	Preset      string `json:"preset"`
	StoragePath string `json:"storage_path"`
}

func defaultJobSettings() jobSettings {
	return jobSettings{Codec: "libx265", CRF: 18, Preset: "medium", StoragePath: ""}
}

// ponytail: no settings table exists yet — these live in memory per server
// process and reset on restart. Add a `settings` table (mirroring the
// runners/jobs pattern) if per-user persistence across restarts is needed.
var (
	settingsMu sync.Mutex
	settings   = map[uuid.UUID]jobSettings{}
)

// HandleGetSettings returns the caller's saved defaults, or hardcoded
// defaults if they haven't set any yet.
func HandleGetSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		settingsMu.Lock()
		s, ok := settings[userID]
		settingsMu.Unlock()
		if !ok {
			s = defaultJobSettings()
		}
		c.JSON(http.StatusOK, s)
	}
}

// HandlePutSettings replaces the caller's saved defaults.
func HandlePutSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		s := defaultJobSettings()
		if err := c.ShouldBindJSON(&s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		settingsMu.Lock()
		settings[userID] = s
		settingsMu.Unlock()

		c.JSON(http.StatusOK, s)
	}
}
