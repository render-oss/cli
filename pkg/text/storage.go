package text

import (
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/table"

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

// ObjectDeleteMultiple formats multiple delete results for text output
func ObjectDeleteMultiple(results []*storage.DeleteResult) string {
	var lines []string
	for _, result := range results {
		lines = append(lines, FormatStringF("Deleted %s", result.Key))
	}
	return strings.Join(lines, "")
}

// ObjectTable formats a list of objects for text output
func ObjectTable(objects []storage.ObjectInfo) string {
	t := newTable()
	t.AppendHeader(table.Row{"KEY", "SIZE", "LAST MODIFIED"})
	for _, obj := range objects {
		t.AppendRow(table.Row{
			obj.Key,
			utils.FormatBytes(obj.SizeBytes),
			obj.LastModified.Format(time.RFC3339),
		})
	}
	return FormatString(t.Render())
}
