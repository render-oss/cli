package testids

import (
	"testing"

	"github.com/render-oss/cli/pkg/validate"
)

func TestResourceIDsAreValid(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		validate func(string) bool
	}{
		{"workspace", WorkspaceID("target workspace"), validate.IsWorkspaceID},
		{"user", UserID("target user"), validate.IsWorkspaceID},
		{"project", ProjectID("my project"), validate.IsProjectID},
		{"environment", EnvironmentID("production"), validate.IsEnvironmentID},
		{"postgres", PostgresID("appdb"), validate.IsPostgresID},
		{"service", ServiceID("api"), validate.IsServiceID},
		{"cron job", CronJobID("daily"), func(s string) bool { return validate.IsObjectID("crn", s) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.validate(tc.id) {
				t.Fatalf("expected valid ID, got %q", tc.id)
			}
		})
	}
}

func TestResourceIDsEmbedSanitizedLabel(t *testing.T) {
	got := ProjectID("Project A!")

	if got != "prj-projecta000000000000" {
		t.Fatalf("ProjectID() = %q", got)
	}
}

func TestRandomResourceIDsAreValidAndUnique(t *testing.T) {
	tests := []struct {
		name     string
		first    string
		second   string
		validate func(string) bool
	}{
		{"workspace", RandomWorkspaceID(), RandomWorkspaceID(), validate.IsWorkspaceID},
		{"user", RandomUserID(), RandomUserID(), validate.IsWorkspaceID},
		{"project", RandomProjectID(), RandomProjectID(), validate.IsProjectID},
		{"environment", RandomEnvironmentID(), RandomEnvironmentID(), validate.IsEnvironmentID},
		{"postgres", RandomPostgresID(), RandomPostgresID(), validate.IsPostgresID},
		{"service", RandomServiceID(), RandomServiceID(), validate.IsServiceID},
		{"cron job", RandomCronJobID(), RandomCronJobID(), func(s string) bool { return validate.IsObjectID("crn", s) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.validate(tc.first) {
				t.Fatalf("expected valid first ID, got %q", tc.first)
			}
			if !tc.validate(tc.second) {
				t.Fatalf("expected valid second ID, got %q", tc.second)
			}
			if tc.first == tc.second {
				t.Fatalf("expected random IDs to differ, got %q", tc.first)
			}
		})
	}
}
