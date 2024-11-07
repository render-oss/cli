package cmd

import "github.com/spf13/cobra"

var (
	GroupCore = &cobra.Group{
		ID:    "core",
		Title: "Core",
	}
	GroupAuth = &cobra.Group{
		ID:    "auth",
		Title: "Auth",
	}
	GroupSession = &cobra.Group{
		ID:    "session",
		Title: "Session",
	}
	GroupManagement = &cobra.Group{
		ID:    "management",
		Title: "Management",
	}

	AllGroups = []*cobra.Group{
		GroupCore,
		GroupAuth,
		GroupSession,
		GroupManagement,
	}
)
