package service

import (
	"fmt"
	"strings"

	types "github.com/render-oss/cli/pkg/types"
)

type ServiceType string

const (
	ServiceTypeWebService       ServiceType = "web_service"
	ServiceTypePrivateService   ServiceType = "private_service"
	ServiceTypeBackgroundWorker ServiceType = "background_worker"
	ServiceTypeStaticSite       ServiceType = "static_site"
	ServiceTypeCronJob          ServiceType = "cron_job"
)

var serviceTypeValues = []ServiceType{
	ServiceTypeWebService,
	ServiceTypePrivateService,
	ServiceTypeBackgroundWorker,
	ServiceTypeStaticSite,
	ServiceTypeCronJob,
}

func ServiceTypeValues() []string {
	values := make([]string, 0, len(serviceTypeValues))
	for _, value := range serviceTypeValues {
		values = append(values, string(value))
	}
	return values
}

func ParseServiceType(value string) (ServiceType, error) {
	normalized := strings.TrimSpace(value)
	for _, serviceType := range serviceTypeValues {
		if normalized == string(serviceType) {
			return serviceType, nil
		}
	}

	return "", fmt.Errorf("type must be one of: %s", strings.Join(ServiceTypeValues(), ", "))
}

func OptionalServiceType[S ~string](value *S) (*ServiceType, error) {
	return types.ParseOptionalString(value, ParseServiceType)
}
