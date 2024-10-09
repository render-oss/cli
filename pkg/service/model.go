package service

import (
	"github.com/renderinc/render-cli/pkg/client"
)

var Types = []string{
	BackgroundWorkerResourceType,
	CronJobResourceType,
	PrivateServiceResourceType,
	StaticSiteResourceType,
	WebServiceResourceType,
}

const (
	BackgroundWorkerResourceType = "Background Worker"
	CronJobResourceType          = "Cron Job"
	PrivateServiceResourceType   = "Private Service"
	StaticSiteResourceType       = "Static Site"
	WebServiceResourceType       = "Web Service"
)

const ServerResourceIDPrefix = "srv-"
const CronjobResourceIDPrefix = "crn-"

type Model struct {
	Service     *client.Service
	Project     *client.Project
	Environment *client.Environment
}

func (s Model) ID() string {
	return s.Service.Id
}

func (s Model) Name() string {
	return s.Service.Name
}

func (s Model) ProjectName() string {
	if s.Project != nil {
		return s.Project.Name
	}
	return ""
}

func (s Model) EnvironmentName() string {
	if s.Environment != nil {
		return s.Environment.Name
	}
	return ""
}

func (s Model) Type() string {
	switch s.Service.Type {
	case client.BackgroundWorker:
		return BackgroundWorkerResourceType
	case client.CronJob:
		return CronJobResourceType
	case client.PrivateService:
		return PrivateServiceResourceType
	case client.StaticSite:
		return StaticSiteResourceType
	case client.WebService:
		return WebServiceResourceType
	default:
		return ""
	}
}
