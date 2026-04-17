package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/v2/pkg/command"
	renderstyle "github.com/render-oss/cli/v2/pkg/style"
	"github.com/render-oss/cli/v2/pkg/workflows/scaffold"
)

// InitDeps abstracts side effects for testability.
type InitDeps interface {
	LocalRepoOverride(lang scaffold.Language) string
	CloneRepo(ctx context.Context, destDir string, lang scaffold.Language) error
	DiscoverTemplates(repoDir string) ([]scaffold.DiscoveredTemplate, error)
	Scaffold(opts scaffold.Options) (*scaffold.Result, error)
	InstallDeps(ctx context.Context, dir string, installCmd string) error
	InitGit(dir string) error
}

// defaultInitDeps delegates to real implementations.
type defaultInitDeps struct{}

func (d *defaultInitDeps) LocalRepoOverride(lang scaffold.Language) string {
	return scaffold.LocalRepoOverride(lang)
}

func (d *defaultInitDeps) CloneRepo(ctx context.Context, destDir string, lang scaffold.Language) error {
	return scaffold.CloneTemplatesRepo(ctx, destDir, lang)
}

func (d *defaultInitDeps) DiscoverTemplates(repoDir string) ([]scaffold.DiscoveredTemplate, error) {
	return scaffold.DiscoverTemplates(repoDir)
}

func (d *defaultInitDeps) Scaffold(opts scaffold.Options) (*scaffold.Result, error) {
	return scaffold.Scaffold(opts)
}

func (d *defaultInitDeps) InstallDeps(ctx context.Context, dir string, installCmd string) error {
	c := exec.CommandContext(ctx, "sh", "-c", installCmd)
	c.Dir = dir

	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf

	if err := c.Run(); err != nil {
		// Include the captured output so the user sees the actual error
		// (e.g. "pip: command not found") instead of just "exit status 127".
		output := strings.TrimSpace(buf.String())
		if output != "" {
			return fmt.Errorf("%s\n%s", err, output)
		}
		return err
	}
	return nil
}

func (d *defaultInitDeps) InitGit(dir string) error {
	return scaffold.InitGitRepo(dir)
}

// WorkflowInitRunner orchestrates the workflows init flow.
type WorkflowInitRunner struct {
	deps        InitDeps
	interactive bool
	cmd         *cobra.Command
}

func truncateDisplayWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	const ellipsis = "…"
	ellipsisWidth := lipgloss.Width(ellipsis)
	if maxWidth <= ellipsisWidth {
		return ellipsis
	}

	available := maxWidth - ellipsisWidth
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > available {
			break
		}
		b.WriteRune(r)
		used += rw
	}

	return b.String() + ellipsis
}

func expandHomePath(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if path != "~" && len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		// Keep "~user/..." unchanged; we only support current-user home expansion.
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// prePrompt prints a blank line before an interactive prompt for visual spacing.
func (r *WorkflowInitRunner) prePrompt() {
	command.Println(r.cmd, "")
}

// postPrompt removes the blank line printed by prePrompt after the prompt
// completes, so consecutive confirmation lines stack tightly.
// Only emits ANSI codes when stdout is a TTY.
func (r *WorkflowInitRunner) postPrompt() {
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		fmt.Fprintf(r.cmd.OutOrStdout(), "\033[A\033[2K")
	}
}

func (r *WorkflowInitRunner) handlePromptError(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		command.Println(r.cmd, "Setup canceled.")
		return nil
	}
	return err
}

// Run executes the full init flow: resolve templates, prompt for options
// (in interactive mode), scaffold, install deps, init git, and print results.
func (r *WorkflowInitRunner) Run(ctx context.Context, input WorkflowInitInput) error {
	// --confirm skips all interactive prompts, using flags/defaults for everything
	skipPrompts := command.GetConfirmFromContext(ctx)

	if (!r.interactive || skipPrompts) && input.Language == "" {
		return fmt.Errorf("--language is required when using --confirm or non-interactive mode")
	}

	// Intro message
	if r.interactive {
		command.Println(r.cmd, "")
		command.Println(r.cmd, "%s", renderstyle.Bold("Initializing Workflows demo project"))
	}

	// Interactive prompt 1: Language
	if r.interactive && !skipPrompts && input.Language == "" {
		r.prePrompt()
		var language string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select a language for your workflow tasks").
					Options(
						huh.NewOption("Node.js (TypeScript)", "node"),
						huh.NewOption("Python", "python"),
					).
					Value(&language),
			),
		)
		if err := form.Run(); err != nil {
			return r.handlePromptError(err)
		}
		r.postPrompt()
		input.Language = language
	}

	lang, err := scaffold.ParseLanguage(input.Language)
	if err != nil {
		return err
	}

	ok := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

	// Confirm language selection
	if r.interactive {
		command.Println(r.cmd, "  %s Language: %s", ok.Render("✓"), lang.DisplayName())
		time.Sleep(200 * time.Millisecond)
	}

	// Resolve templates directory: use a local override if the
	// corresponding env var is set, otherwise clone from GitHub.
	var repoDir string
	if localDir := r.deps.LocalRepoOverride(lang); localDir != "" {
		repoDir = localDir
		if r.interactive {
			command.Println(r.cmd, "  %s Using local templates: %s", ok.Render("✓"), localDir)
			time.Sleep(200 * time.Millisecond)
		}
	} else {
		if r.interactive {
			command.Println(r.cmd, "  %s", dim.Render("Fetching workflow templates..."))
		}

		tmpDir, err := os.MkdirTemp("", "render-workflow-templates-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		cloneCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		if err := r.deps.CloneRepo(cloneCtx, tmpDir, lang); err != nil {
			return err
		}

		repoDir = tmpDir
	}

	// Discover available templates
	templates, err := r.deps.DiscoverTemplates(repoDir)
	if err != nil {
		return err
	}

	if r.interactive {
		// Overwrite the "Fetching workflow templates..." line
		if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			fmt.Fprintf(r.cmd.OutOrStdout(), "\033[A\033[2K")
		}
		command.Println(r.cmd, "  %s Found %d %s",
			ok.Render("✓"), len(templates), pluralize("template", len(templates)))
		time.Sleep(200 * time.Millisecond)
	}

	// Interactive prompt 2: Template (if not provided and >1 available)
	if input.Template == "" {
		if len(templates) == 1 {
			input.Template = templates[0].DirName
		} else if r.interactive && !skipPrompts {
			r.prePrompt()
			const maxTemplateNameWidth = 20
			const templateDescriptionGap = "   "
			templateNameStyle := lipgloss.NewStyle().Bold(true)
			maxTemplateLabelWidth := 0
			for _, t := range templates {
				name := truncateDisplayWidth(t.Name, maxTemplateNameWidth)
				if w := lipgloss.Width(name); w > maxTemplateLabelWidth {
					maxTemplateLabelWidth = w
				}
			}

			var templateOptions []huh.Option[string]
			for _, t := range templates {
				name := truncateDisplayWidth(t.Name, maxTemplateNameWidth)
				label := templateNameStyle.Render(name)
				if t.Description != "" {
					padding := ""
					if pad := maxTemplateLabelWidth - lipgloss.Width(name); pad > 0 {
						padding = strings.Repeat(" ", pad)
					}
					label = fmt.Sprintf("%s%s%s%s", label, padding, templateDescriptionGap, t.Description)
				}
				templateOptions = append(templateOptions, huh.NewOption(label, t.DirName))
			}

			var selected string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Select a starter template").
						Options(templateOptions...).
						Value(&selected),
				),
			)
			if err := form.Run(); err != nil {
				return r.handlePromptError(err)
			}
			r.postPrompt()
			input.Template = selected
		} else {
			// Non-interactive: default to first template (typically hello-world)
			input.Template = templates[0].DirName
		}
	}

	// Find display name and description for selected template
	templateDisplayName := input.Template
	templateDescription := ""
	for _, t := range templates {
		if t.DirName == input.Template {
			templateDisplayName = t.Name
			templateDescription = t.Description
			break
		}
	}
	if r.interactive {
		command.Println(r.cmd, "")
		command.Println(r.cmd, "  %s Template: %s", ok.Render("✓"), templateDisplayName)
		if templateDescription != "" {
			command.Println(r.cmd, "    %s", dim.Render(templateDescription))
		}
		command.Println(r.cmd, "")
	}

	// Interactive prompt 3: Output directory
	// In interactive mode, re-prompt if the chosen directory is not empty.
	if r.interactive && !skipPrompts {
		if input.Dir == "" {
			input.Dir = defaultDir
		}

		warn := lipgloss.NewStyle().Foreground(renderstyle.ColorWarning)
		firstAttempt := true
		for {
			if !firstAttempt {
				r.prePrompt()
			}
			firstAttempt = false
			dir := input.Dir
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Specify a project directory (must be new or empty)").
						Value(&dir),
				),
			)
			if err := form.Run(); err != nil {
				return r.handlePromptError(err)
			}
			r.postPrompt()
			input.Dir = dir

			expandedDir, err := expandHomePath(input.Dir)
			if err != nil {
				return err
			}
			absDir, err := filepath.Abs(expandedDir)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}

			if entries, err := os.ReadDir(absDir); err == nil && len(entries) > 0 {
				command.Println(r.cmd, "  %s Directory %s already exists and is not empty.",
					warn.Render("!"), input.Dir)
				continue
			}
			break
		}
	}

	if input.Dir == "" {
		input.Dir = defaultDir
	}

	if r.interactive {
		command.Println(r.cmd, "  %s Directory: %s", ok.Render("✓"), input.Dir)
	}

	input.Dir = strings.TrimPrefix(input.Dir, "./")

	expandedDir, err := expandHomePath(input.Dir)
	if err != nil {
		return err
	}
	absDir, err := filepath.Abs(expandedDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// When prompts are skipped (--confirm or non-interactive), fail on non-empty directory.
	// In interactive mode without --confirm, the prompt loop handles re-prompting.
	if !r.interactive || skipPrompts {
		if entries, err := os.ReadDir(absDir); err == nil && len(entries) > 0 {
			return fmt.Errorf("directory %s already exists and is not empty", input.Dir)
		}
	}

	// Interactive prompt 4: Install dependencies
	wantDeps := input.InstallDeps
	if r.interactive && !skipPrompts {
		r.prePrompt()
		var installDeps string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Install project dependencies?").
					Description("(recommended)").
					Options(
						huh.NewOption("Yes", "yes"),
						huh.NewOption("No", "no"),
					).
					Value(&installDeps),
			),
		)
		if err := form.Run(); err != nil {
			return r.handlePromptError(err)
		}
		r.postPrompt()
		wantDeps = installDeps == "yes"
		if wantDeps {
			command.Println(r.cmd, "  %s Dependencies: yes", ok.Render("✓"))
		} else {
			command.Println(r.cmd, "  %s Dependencies: no", ok.Render("✓"))
		}
	} else if r.interactive {
		if wantDeps {
			command.Println(r.cmd, "  %s Dependencies: yes", ok.Render("✓"))
		} else {
			command.Println(r.cmd, "  %s Dependencies: no", ok.Render("✓"))
		}
	}

	// Interactive prompt 5: Initialize git repository
	wantGit := input.Git
	if r.interactive && !skipPrompts {
		gitDir := filepath.Join(absDir, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			r.prePrompt()
			var initGit string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Initialize a new Git repository?").
						Description("(recommended)").
						Options(
							huh.NewOption("Yes", "yes"),
							huh.NewOption("No", "no"),
						).
						Value(&initGit),
				),
			)
			if err := form.Run(); err != nil {
				return r.handlePromptError(err)
			}
			r.postPrompt()
			wantGit = initGit == "yes"
			if wantGit {
				command.Println(r.cmd, "  %s Git: yes", ok.Render("✓"))
			} else {
				command.Println(r.cmd, "  %s Git: no", ok.Render("✓"))
			}
		}
	} else if r.interactive && wantGit {
		command.Println(r.cmd, "  %s Git: yes", ok.Render("✓"))
	}

	// Build and execute setup steps
	var result *scaffold.Result
	var depsInstalled, gitInitialized bool

	var steps []SetupStep

	// Step 1: Scaffold (critical step: deps and git depend on scaffolded files)
	steps = append(steps, SetupStep{
		Label:       "Template copied",
		FailLabel:   "Template copy failed",
		ActiveLabel: "Copying template...",
		Critical:    true,
		Run: func() error {
			var err error
			result, err = r.deps.Scaffold(scaffold.Options{
				Language:     lang,
				TemplateName: input.Template,
				Dir:          absDir,
				RepoDir:      repoDir,
			})
			return err
		},
	})

	// Step 2: Install dependencies (optional)
	var depsCommand string // captures the command that was run, for display
	if wantDeps {
		steps = append(steps, SetupStep{
			Label:       "Dependencies installed",
			FailLabel:   "Dependency installation failed",
			ActiveLabel: "Installing dependencies...",
			Detail:      func() string { return depsCommand },
			Run: func() error {
				depsCommand = scaffold.DepsInstallCommand(result.Language, result.BuildCommand)
				if err := r.deps.InstallDeps(ctx, absDir, depsCommand); err != nil {
					return err
				}
				depsInstalled = true
				return nil
			},
		})
	}

	// Step 3: Git init (optional)
	if wantGit {
		steps = append(steps, SetupStep{
			Label:       "Git repository initialized",
			FailLabel:   "Git initialization failed",
			ActiveLabel: "Initializing Git repository...",
			Run: func() error {
				gitDir := filepath.Join(absDir, ".git")
				if _, statErr := os.Stat(gitDir); !os.IsNotExist(statErr) {
					return nil // already has .git, skip
				}
				if err := r.deps.InitGit(absDir); err != nil {
					return err
				}
				gitInitialized = true
				return nil
			},
		})
	}

	// Choose observer and error behavior based on mode
	var obs StepObserver
	if r.interactive {
		obs = newChecklistObserver(r.cmd, steps)
	} else {
		obs = newSilentObserver(r.cmd)
	}

	if err := RunSteps(steps, obs, !r.interactive); err != nil {
		return err
	}

	// Non-interactive summary
	if !r.interactive {
		command.Println(r.cmd, "  %s Scaffolded project at %s", ok.Render("✓"), input.Dir)
		if depsInstalled {
			command.Println(r.cmd, "  %s Dependencies installed", ok.Render("✓"))
		}
		if gitInitialized {
			command.Println(r.cmd, "  %s Initialized Git repository", ok.Render("✓"))
		}
	}

	// Agent skill installation (optional, before next steps)
	if r.interactive && !skipPrompts {
		promptSkillInstall(r.cmd)
	} else if input.InstallAgentSkill {
		installSkillNonInteractive(r.cmd)
	}

	// Brief pause before next steps output
	if r.interactive {
		time.Sleep(400 * time.Millisecond)
	}
	command.Println(r.cmd, "%s", formatNextSteps(result, input.Dir))

	return nil
}
