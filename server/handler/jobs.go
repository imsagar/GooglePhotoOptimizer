package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/relay"
	"github.com/user/gpoptimizer/server/store"
)

const bulkUploadPageSize = 10000 // ponytail: single page covers MVP scale; paginate if a user ever has more ready jobs than this

type createJobsRequest struct {
	VideoIDs []string `json:"video_ids" binding:"required"`
	Codec    string   `json:"codec"`
	CRF      int      `json:"crf"`
	Preset   string   `json:"preset"`
}

// HandleCreateJobs queues an optimize job per video and tells the runner to
// start downloading each one.
func HandleCreateJobs(db *store.DB, r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		var req createJobsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		runDate := time.Now().Format("2006-01-02")
		params := make([]store.CreateJobParams, len(req.VideoIDs))
		for i, vid := range req.VideoIDs {
			params[i] = store.CreateJobParams{
				VideoID: vid,
				Codec:   req.Codec,
				CRF:     req.CRF,
				Preset:  req.Preset,
				RunDate: runDate,
			}
		}

		jobs, err := db.CreateJobs(c.Request.Context(), userID, params)
		if err != nil {
			log.Printf("jobs: create failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		for _, j := range jobs {
			baseURL, _ := db.GetVideoBaseURL(c.Request.Context(), userID, j.VideoID)
			cmd, err := json.Marshal(protocol.Command{
				Type:    "download",
				VideoID: j.VideoID,
				BaseURL: baseURL,
				JobID:   j.ID,
				Codec:   j.Codec,
				CRF:     j.CRF,
				Preset:  j.Preset,
			})
			if err != nil {
				continue
			}
			r.SendToRunner(userID, cmd)
		}

		c.JSON(http.StatusCreated, gin.H{"jobs": jobs})
	}
}

func parseListJobsParams(c *gin.Context) store.ListJobsParams {
	p := store.ListJobsParams{Status: c.Query("status")}
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		p.Page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil {
		p.PageSize = v
	}
	return p
}

// HandleListJobs returns a paginated list of the user's jobs, optionally
// filtered by status.
func HandleListJobs(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		params := parseListJobsParams(c)
		jobs, total, err := db.ListJobs(c.Request.Context(), userID, params)
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
			"data":      jobs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

func jobIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return 0, false
	}
	return id, true
}

// HandleGetJob returns a single job, scoped to the caller.
func HandleGetJob(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		jobID, ok := jobIDParam(c)
		if !ok {
			return
		}

		job, err := db.GetJob(c.Request.Context(), jobID, userID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, job)
	}
}

// HandleCancelJob marks a job cancelled and tells the runner to stop working on it.
func HandleCancelJob(db *store.DB, r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		jobID, ok := jobIDParam(c)
		if !ok {
			return
		}

		if _, err := db.GetJob(c.Request.Context(), jobID, userID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if err := db.UpdateJobStatus(c.Request.Context(), jobID, userID, "cancelled", 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		cmd, err := json.Marshal(protocol.Command{Type: "cancel", JobID: jobID})
		if err == nil {
			r.SendToRunner(userID, cmd)
		}
		c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
	}
}

type uploadRequest struct {
	DeleteOriginal bool `json:"delete_original"`
}

// HandleUploadJob tells the runner to upload one finished job's optimized
// file back to Google Photos.
func HandleUploadJob(db *store.DB, r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		jobID, ok := jobIDParam(c)
		if !ok {
			return
		}

		job, err := db.GetJob(c.Request.Context(), jobID, userID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		var req uploadRequest
		_ = c.ShouldBindJSON(&req) // optional body; default delete_original=false overrides nothing if absent
		deleteOriginal := req.DeleteOriginal || job.DeleteOriginal

		cmd, err := json.Marshal(protocol.Command{Type: "upload", JobID: job.ID, VideoID: job.VideoID, RunDate: job.RunDate, DeleteOriginal: deleteOriginal})
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		r.SendToRunner(userID, cmd)
		c.JSON(http.StatusAccepted, gin.H{"status": "requested"})
	}
}

// HandleBulkUpload uploads every job currently sitting in "ready" status.
func HandleBulkUpload(db *store.DB, r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := auth.UserID(c)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		jobs, _, err := db.ListJobs(c.Request.Context(), userID, store.ListJobsParams{
			Status:   "ready",
			PageSize: bulkUploadPageSize,
		})
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		for _, j := range jobs {
			cmd, err := json.Marshal(protocol.Command{Type: "upload", JobID: j.ID, VideoID: j.VideoID, RunDate: j.RunDate, DeleteOriginal: j.DeleteOriginal})
			if err != nil {
				continue
			}
			r.SendToRunner(userID, cmd)
		}

		c.JSON(http.StatusAccepted, gin.H{"status": "requested", "count": len(jobs)})
	}
}
