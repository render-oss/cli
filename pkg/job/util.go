package job

import client "github.com/render-oss/cli/v2/pkg/client/jobs"

// IsCancellable returns true if the job is cancellable. JobStatus only contains terminal values, so
// a nil value indicates that the job is cancellable.
func IsCancellable(status *client.JobStatus) bool {
	if status == nil {
		return true
	}
	return *status != client.Canceled && *status != client.Failed && *status != client.Succeeded
}
