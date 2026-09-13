package google

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/oauth2"
)

type PhotosClient struct {
	http *http.Client
}

func NewPhotosClient(token *oauth2.Token, cfg *oauth2.Config) *PhotosClient {
	return &PhotosClient{http: cfg.Client(context.Background(), token)}
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
