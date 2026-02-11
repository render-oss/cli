package cmd

import (
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:     "skills",
	Short:   "Manage Render agent skills for AI coding tools",
	GroupID: GroupManagement.ID,
	Long: `Install and manage Render agent skills for AI coding tools such as
Claude Code, Codex, OpenCode, and Cursor.

Skills add deployment, debugging, and monitoring capabilities to your
AI coding assistant.`,
}

func init() {
	rootCmd.AddCommand(skillsCmd)
}
