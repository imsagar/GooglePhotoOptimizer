package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/relay"
	"github.com/user/gpoptimizer/server/store"
)

// NewRouter wires the Google auth routes, REST API handlers and the
// WebSocket hub endpoints.
func NewRouter(db *store.DB, r relay.Relay, authCfg auth.Config) *gin.Engine {
	router := gin.Default()

	auth.Routes(router, db, authCfg, r)

	// Runner registration is public: the runner has no session yet, only a
	// pairing code obtained by the (already-authenticated) UI.
	router.POST("/api/runner/register", HandleRegister(db))

	api := router.Group("/api", auth.Middleware())

	api.POST("/runner/pair", HandlePair(db))
	api.POST("/runner/rotate", HandleRotate(db))

	api.GET("/videos", HandleListVideos(db))
	api.POST("/photos/picker/start", HandlePickerStart(db, authCfg))
	api.GET("/photos/picker/poll", HandlePickerPoll(db, authCfg))

	api.POST("/jobs", HandleCreateJobs(db, r))
	api.GET("/jobs", HandleListJobs(db))
	api.GET("/jobs/:id", HandleGetJob(db))
	api.POST("/jobs/:id/cancel", HandleCancelJob(db, r))
	api.POST("/jobs/:id/upload", HandleUploadJob(db, r))
	api.POST("/jobs/upload-all", HandleBulkUpload(db, r))
	api.DELETE("/jobs", HandleClearJobs(db))

	api.GET("/settings", HandleGetSettings())
	api.PUT("/settings", HandlePutSettings())

	api.GET("/cleanup/local-files", HandleListLocalFiles(r))
	api.POST("/cleanup/local-files/delete", HandleDeleteLocalFiles(r))

	router.GET("/ws/ui", auth.Middleware(), HandleUIWebSocket(db, r))
	router.GET("/ws/runner", HandleRunnerWebSocket(db, r, authCfg))

	return router
}
