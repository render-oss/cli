package service

import "github.com/renderinc/render-cli/pkg/client"

type Model struct {
	*client.Service
	*client.Project
	*client.Environment
}
