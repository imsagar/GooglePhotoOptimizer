package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
)

const photosSearchURL = "https://photoslibrary.googleapis.com/v1/mediaItems:search"

type PhotosClient struct {
	http *http.Client
}

type VideoMeta struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mimeType"`
	BaseURL      string `json:"baseUrl"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CreationTime string `json:"creationTime"`
	// metadata.video.fps, processingStatus
}

// searchResponse mirrors the subset of mediaItems:search's response we need.
type searchResponse struct {
	MediaItems []struct {
		ID            string `json:"id"`
		Filename      string `json:"filename"`
		MimeType      string `json:"mimeType"`
		BaseURL       string `json:"baseUrl"`
		MediaMetadata struct {
			Width        string `json:"width"`
			Height       string `json:"height"`
			CreationTime string `json:"creationTime"`
		} `json:"mediaMetadata"`
	} `json:"mediaItems"`
	NextPageToken string `json:"nextPageToken"`
}

func NewPhotosClient(token *oauth2.Token, cfg *oauth2.Config) *PhotosClient {
	return &PhotosClient{http: cfg.Client(context.Background(), token)}
}

// ListVideos fetches every video in the user's library, following
// nextPageToken until the API reports no more pages.
func (c *PhotosClient) ListVideos(ctx context.Context) ([]VideoMeta, error) {
	var all []VideoMeta
	pageToken := ""
	for {
		reqBody := map[string]interface{}{
			"pageSize": 100,
			"filters": map[string]interface{}{
				"mediaTypeFilter": map[string]interface{}{
					"mediaTypes": []string{"VIDEO"},
				},
			},
		}
		if pageToken != "" {
			reqBody["pageToken"] = pageToken
		}
		body, err := json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, photosSearchURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("photos search failed: %s: %s", resp.Status, respBody)
		}

		var parsed searchResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("decode photos search response: %w", err)
		}

		for _, item := range parsed.MediaItems {
			if !strings.HasPrefix(item.MimeType, "video/") {
				continue
			}
			var width, height int
			fmt.Sscanf(item.MediaMetadata.Width, "%d", &width)
			fmt.Sscanf(item.MediaMetadata.Height, "%d", &height)
			all = append(all, VideoMeta{
				ID:           item.ID,
				Filename:     item.Filename,
				MimeType:     item.MimeType,
				BaseURL:      item.BaseURL,
				Width:        width,
				Height:       height,
				CreationTime: item.MediaMetadata.CreationTime,
			})
		}

		if parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}
	return all, nil
}

func (c *PhotosClient) DownloadVideo(ctx context.Context, baseURL, destPath string, resumeFrom int64) error {
	dlURL := baseURL + "=dv"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return err
	}
	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: %s: %s", resp.Status, body)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resumeFrom > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(destPath, flags, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
