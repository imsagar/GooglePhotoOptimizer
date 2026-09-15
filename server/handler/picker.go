package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/store"
)

const pickerBase = "https://photospicker.googleapis.com/v1"

func pickerClient(db *store.DB, cfg auth.Config, c *gin.Context) (*http.Client, error) {
	userID, err := auth.UserID(c)
	if err != nil {
		return nil, err
	}
	tokJSON, err := db.GetGoogleToken(c.Request.Context(), userID)
	if err != nil || tokJSON == "" {
		return nil, fmt.Errorf("not connected to Google Photos")
	}
	var tok oauth2.Token
	if err := json.Unmarshal([]byte(tokJSON), &tok); err != nil {
		return nil, err
	}
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
	}
	return oauthCfg.Client(c.Request.Context(), &tok), nil
}

func HandlePickerStart(db *store.DB, cfg auth.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := pickerClient(db, cfg, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, err := client.Post(pickerBase+"/sessions", "application/json", nil)
		if err != nil {
			log.Printf("picker: create session request failed: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create picker session"})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			log.Printf("picker: create session failed (%d): %s", resp.StatusCode, body)
			c.JSON(http.StatusBadGateway, gin.H{"error": string(body)})
			return
		}

		var session struct {
			ID        string `json:"id"`
			PickerURI string `json:"pickerUri"`
		}
		if err := json.Unmarshal(body, &session); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid picker response"})
			return
		}

		userID, _ := auth.UserID(c)
		_ = db.SavePickerSession(c.Request.Context(), userID, session.ID)

		c.JSON(http.StatusOK, gin.H{
			"session_id": session.ID,
			"picker_url": session.PickerURI + "/autoclose",
		})
	}
}

func HandlePickerPoll(db *store.DB, cfg auth.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
			return
		}

		client, err := pickerClient(db, cfg, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, err := client.Get(pickerBase + "/sessions/" + sessionID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to poll picker session"})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadGateway, gin.H{"error": string(body)})
			return
		}

		var session struct {
			MediaItemsSet bool `json:"mediaItemsSet"`
		}
		if err := json.Unmarshal(body, &session); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid session response"})
			return
		}

		if !session.MediaItemsSet {
			c.JSON(http.StatusOK, gin.H{"ready": false})
			return
		}

		items, err := fetchPickerItems(client, sessionID)
		if err != nil {
			log.Printf("picker: fetch items failed: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		userID, _ := auth.UserID(c)
		var videos []store.Video
		for _, item := range items {
			if item.Type != "VIDEO" {
				continue
			}
			v := store.Video{
				ID:       item.ID,
				Filename: item.MediaFile.Filename,
				MimeType: item.MediaFile.MimeType,
				BaseURL:  item.MediaFile.BaseURL,
			}
			if t, err := time.Parse(time.RFC3339, item.CreateTime); err == nil {
				v.CreationTime = &t
			}
			v.Width = item.MediaFile.Metadata.Width
			v.Height = item.MediaFile.Metadata.Height
			videos = append(videos, v)
		}

		if len(videos) > 0 {
			_ = db.UpsertVideos(c.Request.Context(), userID, videos)
		}

		c.JSON(http.StatusOK, gin.H{"ready": true, "count": len(videos)})
	}
}

type pickerItem struct {
	ID string `json:"id"`
	// CreateTime is the media's capture date. The Picker API returns it at
	// the item's top level, NOT inside mediaFileMetadata.
	CreateTime string `json:"createTime"`
	Type       string `json:"type"`
	MediaFile  struct {
		BaseURL  string `json:"baseUrl"`
		MimeType string `json:"mimeType"`
		Filename string `json:"filename"`
		Metadata struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"mediaFileMetadata"`
	} `json:"mediaFile"`
}

func fetchPickerItems(client *http.Client, sessionID string) ([]pickerItem, error) {
	var all []pickerItem
	pageToken := ""
	for {
		url := fmt.Sprintf("%s/mediaItems?sessionId=%s&pageSize=100", pickerBase, sessionID)
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}
		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch items failed: %s", body)
		}

		var result struct {
			MediaItems    []pickerItem `json:"mediaItems"`
			NextPageToken string       `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		all = append(all, result.MediaItems...)
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	return all, nil
}
