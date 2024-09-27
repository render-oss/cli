package service

import "github.com/renderinc/render-cli/pkg/client"

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
		return "Background Worker"
	case client.CronJob:
		return "Cron Job"
	case client.PrivateService:
		return "Private Service"
	case client.StaticSite:
		return "Static Site"
	case client.WebService:
		return "Web Service"
	default:
		return ""
	}
}
