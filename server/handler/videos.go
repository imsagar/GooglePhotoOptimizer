package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/relay"
	"github.com/user/gpoptimizer/server/store"
)

// parseListVideosParams reads sort/order/page/page_size/min_size/from/to/
// album/status query params. Malformed numeric/date values are ignored
// (fall back to store defaults) rather than rejected — these are all
// optional filters.
func parseListVideosParams(c *gin.Context) store.ListVideosParams {
	p := store.ListVideosParams{
		Sort:    c.Query("sort"),
		Order:   c.Query("order"),
		AlbumID: c.Query("album"),
		Status:  c.Query("status"),
	}
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		p.Page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil {
		p.PageSize = v
	}
	if v, err := strconv.ParseInt(c.Query("min_size"), 10, 64); err == nil {
		p.MinSize = v
	}
	if v, err := time.Parse(time.RFC3339, c.Query("from")); err == nil {
		p.From = &v
	}
	if v, err := time.Parse(time.RFC3339, c.Query("to")); err == nil {
		p.To = &v
	}
	return p
}

// HandleListVideos returns a paginated, filtered list of the user's synced videos.
func HandleListVideos(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		params := parseListVideosParams(c)
		videos, total, err := db.ListVideos(c.Request.Context(), userID, params)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		page := params.Page
		if page < 1 {
			page = 1
		}
		pageSize := params.PageSize
		if pageSize < 1 {
			pageSize = 50
		}
		c.JSON(http.StatusOK, gin.H{
			"videos":    videos,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// HandleSyncVideos asks the user's runner to re-scan Google Photos and push
// updated video metadata back over /ws/runner (handled as a "videos_synced"
// status in ws.go). Fire-and-forget: the request just queues the command.
func HandleSyncVideos(r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		cmd, err := json.Marshal(protocol.Command{Type: "sync_videos"})
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		r.SendToRunner(userID, cmd)
		c.JSON(http.StatusAccepted, gin.H{"status": "requested"})
	}
}
