package job

import client "github.com/renderinc/render-cli/pkg/client/jobs"


// IsCancellable returns true if the job is cancellable. JobStatus only contains terminal values, so
// a nil value indicates that the job is cancellable.
func IsCancellable(status *client.JobStatus) bool {
	return status == nil
}
