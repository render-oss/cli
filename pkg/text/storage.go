package text

import (
	"github.com/render-oss/cli/pkg/storage"
	"github.com/render-oss/cli/pkg/utils"
)

// ObjectUpload formats an upload result for text output
func ObjectUpload(result *storage.UploadResult) string {
	sizeStr := utils.FormatBytes(result.SizeBytes)
	if result.LocalPath != "" {
		return FormatStringF("Uploaded %s to %s (%s)", result.LocalPath, result.Key, sizeStr)
	}
	return FormatStringF("Uploaded to %s (%s)", result.Key, sizeStr)
}

// ObjectDownload formats a download result for text output
func ObjectDownload(result *storage.DownloadResult) string {
	sizeStr := utils.FormatBytes(result.SizeBytes)
	if result.LocalPath != "" {
		return FormatStringF("Downloaded %s to %s (%s)", result.Key, result.LocalPath, sizeStr)
	}
	return FormatStringF("Downloaded %s (%s)", result.Key, sizeStr)
}

// ObjectDelete formats a delete result for text output
func ObjectDelete(result *storage.DeleteResult) string {
	return FormatStringF("Deleted %s", result.Key)
}
