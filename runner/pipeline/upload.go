package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/google"
)

// HandleUpload uploads the encoded file to Drive, verifies the upload by
// comparing sizes, and optionally deletes the matching original file from
// Drive. Reports progress and a final upload_complete/error status via
// sendStatus.
//
// ponytail: verification is size-only; Drive doesn't expose video duration
// without an extra metadata fetch. Add a duration check if size collisions
// turn out to be a real risk.
func HandleUpload(ctx context.Context, sendStatus func(protocol.Status), drv *google.DriveClient, optimizedPath string, originalFilename string, createdTime string, originalSize int64, jobID int, deleteOriginal bool) error {
	sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "uploading", Percent: 0})

	newFileID, err := drv.Upload(ctx, optimizedPath, originalFilename, createdTime)
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("upload: %v", err)})
		return err
	}

	uploadedSize, err := drv.GetFileMeta(ctx, newFileID)
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: "verification failed: cannot read uploaded file"})
		return err
	}

	localInfo, err := os.Stat(optimizedPath)
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("upload: %v", err)})
		return err
	}
	if uploadedSize != localInfo.Size() {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("size mismatch: local=%d uploaded=%d", localInfo.Size(), uploadedSize)})
		return fmt.Errorf("size verification failed")
	}

	result := protocol.Status{
		Type:         "upload_complete",
		JobID:        jobID,
		DriveFileID:  newFileID,
		SizeVerified: true,
	}

	if deleteOriginal {
		origFileID, err := drv.FindFile(ctx, originalFilename, originalSize)
		if err == nil {
			if err := drv.Delete(ctx, origFileID); err == nil {
				result.OriginalDeleted = true
			}
		}
	}

	sendStatus(result)
	return nil
}
