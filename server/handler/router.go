package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/relay"
	"github.com/user/gpoptimizer/server/store"
)

// NewRouter wires the Google auth routes and the WebSocket hub endpoints.
// REST resource handlers (videos, jobs, runner pairing, settings) are
// registered onto the /api group by Task 6.
func NewRouter(db *store.DB, r relay.Relay, authCfg auth.Config) *gin.Engine {
	router := gin.Default()

	auth.Routes(router, db, authCfg)

	// Auth-protected API group stub; Task 6 adds handlers here.
	router.Group("/api", auth.Middleware())

	router.GET("/ws/ui", auth.Middleware(), HandleUIWebSocket(r))
	router.GET("/ws/runner", HandleRunnerWebSocket(db, r))

	return router
}
