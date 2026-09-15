package google

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type DriveClient struct {
	svc *drive.Service
}

func NewDriveClient(token *oauth2.Token, cfg *oauth2.Config) (*DriveClient, error) {
	client := cfg.Client(context.Background(), token)
	svc, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &DriveClient{svc: svc}, nil
}

// escapeQueryValue escapes single quotes for the Drive API's query language,
// which has no parameterized query support: https://developers.google.com/drive/api/guides/ref-search-terms
func escapeQueryValue(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func (c *DriveClient) FindFile(ctx context.Context, filename string, sizeBytes int64) (string, error) {
	q := fmt.Sprintf("name = '%s' and trashed = false", escapeQueryValue(filename))
	list, err := c.svc.Files.List().Q(q).Fields("files(id, name, size)").Context(ctx).Do()
	if err != nil {
		return "", err
	}
	for _, f := range list.Files {
		if f.Size == sizeBytes {
			return f.Id, nil
		}
	}
	return "", fmt.Errorf("file not found: %s (%d bytes)", filename, sizeBytes)
}

func (c *DriveClient) Upload(ctx context.Context, filePath, filename, createdTime string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	meta := &drive.File{Name: filename}
	if createdTime != "" {
		meta.CreatedTime = createdTime
		meta.ModifiedTime = createdTime
	}
	file, err := c.svc.Files.Create(meta).
		Media(f).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return file.Id, nil
}

func (c *DriveClient) Delete(ctx context.Context, fileID string) error {
	return c.svc.Files.Delete(fileID).Context(ctx).Do()
}

func (c *DriveClient) GetFileMeta(ctx context.Context, fileID string) (int64, error) {
	f, err := c.svc.Files.Get(fileID).Fields("size").Context(ctx).Do()
	if err != nil {
		return 0, err
	}
	return f.Size, nil
}
