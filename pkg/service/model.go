package service

import (
	"github.com/renderinc/render-cli/pkg/client"
)

const (
	BackgroundWorkerResourceType = "BackgroundWorker"
	CronJobResourceType          = "CronJob"
	PrivateServiceResourceType   = "PrivateService"
	StaticSiteResourceType       = "StaticSite"
	WebServiceResourceType       = "WebService"
)

const ServerResourceIDPrefix = "srv-"
const CronjobResourceIDPrefix = "crn-"

type Model struct {
	service     *client.Service
	project     *client.Project
	environment *client.Environment
}

func (s Model) ID() string {
	return s.service.Id
}

func (s Model) Name() string {
	return s.service.Name
}

func (s Model) Service() *client.Service {
	return s.service
}

func (s Model) Project() *client.Project {
	return s.project
}

func (s Model) ProjectName() string {
	if s.project != nil {
		return s.project.Name
	}
	return ""
}

func (s Model) Environment() *client.Environment {
	return s.environment
}

func (s Model) EnvironmentName() string {
	if s.environment != nil {
		return s.environment.Name
	}
	return ""
}

func (s Model) Type() string {
	switch s.service.Type {
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
