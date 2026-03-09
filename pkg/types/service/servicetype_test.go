package service_test

import (
	"testing"

	servicetypes "github.com/render-oss/cli/pkg/types/service"
	"github.com/stretchr/testify/require"
)

func TestParseServiceType(t *testing.T) {
	parsed, err := servicetypes.ParseServiceType("web_service")
	require.NoError(t, err)
	require.Equal(t, servicetypes.ServiceTypeWebService, parsed)

	_, err = servicetypes.ParseServiceType("invalid")
	require.Error(t, err)
}

func TestServiceCreateTypeValues(t *testing.T) {
	values := servicetypes.ServiceTypeValues()
	require.Equal(t, []string{
		"web_service",
		"private_service",
		"background_worker",
		"static_site",
		"cron_job",
	}, values)
}
