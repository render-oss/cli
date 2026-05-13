package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	petname "github.com/dustinkirkland/golang-petname"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resolve"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
	"github.com/render-oss/cli/pkg/validate"
)

// KeyValueCreateModel is an inline interactive wizard for `render kv create`.
// The outer tea.Model owns step orchestration and async data loads; each
// individual step is rendered by a huh.Form composed as a child tea.Model.
// Unlike other views in this package, it is not pushed onto the shared TUI
// stack: RunKeyValueCreate runs a dedicated inline tea.Program (no alt-screen)
// so the wizard preserves shell scrollback.

type kvStep int

const (
	stepInit kvStep = iota
	stepLoadingWorkspaces
	stepWorkspace
	stepName
	stepPlan
	stepRegion
	stepMemoryPolicy
	stepLoadingProjects
	stepProject
	stepLoadingEnvironments
	stepEnvironment
	stepConfirm
	stepCreating
	stepDone
)

type completedStep struct {
	label string
	value string
}

type keyValueCreateRepos struct {
	owners   *owner.Repo
	projects *project.Repo
	envs     *environment.Repo
	resolver *resolve.Resolver
}

type KeyValueCreateModel struct {
	ctx   context.Context
	repos keyValueCreateRepos
	input kvtypes.KeyValueCreateInput

	step      kvStep
	completed []completedStep

	// Active step's UI, as a composed huh form.
	form         *huh.Form
	labelByValue map[string]string

	// Form-bound values (huh writes into these via .Value(&...)).
	selectValue     string
	nameValue       string
	namePlaceholder string
	confirmValue    bool

	// Async / loading state.
	spinner    spinner.Model
	loadingMsg string

	// Outcomes.
	canceled bool
	err      error
	result   *client.KeyValueDetail
}

// --- async messages ---

type workspacesLoadedMsg struct {
	owners []*client.Owner
	err    error
}
type workspaceResolvedMsg struct {
	workspaceID string
	err         error
}
type projectsLoadedMsg struct {
	projects []*client.Project
	err      error
}
type environmentsLoadedMsg struct {
	envs []*client.Environment
	err  error
}
type kvCreateDoneMsg struct {
	kv  *client.KeyValueDetail
	err error
}

// RunKeyValueCreate runs the interactive wizard inline (no alt-screen) and
// returns the created Key Value detail, or nil if the user canceled.
func RunKeyValueCreate(cmd *cobra.Command, input *kvtypes.KeyValueCreateInput) (*client.KeyValueDetail, error) {
	ctx := cmd.Context()

	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	repos := keyValueCreateRepos{
		owners:   owner.NewRepo(c),
		projects: project.NewRepo(c),
		envs:     environment.NewRepo(c),
		resolver: resolve.New(c),
	}

	// Resolve any pre-supplied scope so the wizard skips matching steps.
	var preCompleted []completedStep
	if input.WorkspaceIDOrName != "" || input.ProjectIDOrName != nil || input.EnvironmentIDOrName != nil {
		scope, err := repos.resolver.ResolveScope(ctx, resolve.ScopeInput{
			WorkspaceIDOrName:   input.WorkspaceIDOrName,
			ProjectIDOrName:     input.ProjectIDOrName,
			EnvironmentIDOrName: input.EnvironmentIDOrName,
		})
		if err != nil {
			return nil, err
		}
		input.WorkspaceIDOrName = scope.WorkspaceID
		if scope.Project != nil {
			pid := scope.Project.Id
			input.ProjectIDOrName = &pid
		}
		if scope.Environment != nil {
			eid := scope.Environment.Id
			input.EnvironmentIDOrName = &eid
		}

		// Breadcrumbs for fields we just skipped so the user sees what was resolved.
		if scope.WorkspaceID != "" {
			if o, err := repos.owners.RetrieveOwner(ctx, scope.WorkspaceID); err == nil && o != nil {
				preCompleted = append(preCompleted, completedStep{"Workspace", o.Name})
			}
		}
		if scope.Project != nil {
			preCompleted = append(preCompleted, completedStep{"Project", scope.Project.Name})
		}
		if scope.Environment != nil {
			preCompleted = append(preCompleted, completedStep{"Environment", scope.Environment.Name})
		}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := KeyValueCreateModel{
		ctx:       ctx,
		repos:     repos,
		input:     *input,
		step:      stepInit,
		spinner:   sp,
		completed: preCompleted,
	}

	// Header — printed once, stays in scrollback above the live region.
	command.Println(cmd, "")
	command.Println(cmd, "%s", renderstyle.Bold("Creating Key Value store"))

	p := tea.NewProgram(&m, tea.WithContext(ctx), tea.WithOutput(cmd.OutOrStdout()))
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := finalModel.(*KeyValueCreateModel)
	if fm.err != nil {
		return nil, fm.err
	}
	if fm.canceled {
		return nil, nil
	}
	*input = fm.input
	return fm.result, nil
}

// --- Init / Update / View ---

func (m *KeyValueCreateModel) Init() tea.Cmd {
	return m.advance()
}

func (m *KeyValueCreateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global hotkey: always allow Ctrl+C to bail out.
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyCtrlC {
		m.canceled = true
		return m, tea.Quit
	}

	// Spinner animation while we wait on async work.
	if _, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Domain-specific async results.
	switch msg := msg.(type) {
	case workspacesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m, m.buildWorkspaceForm(msg.owners)

	case workspaceResolvedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.input.WorkspaceIDOrName = msg.workspaceID
		return m, m.afterWorkspace()

	case projectsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		if len(msg.projects) == 0 {
			return m, m.afterEnvironment()
		}
		return m, m.buildProjectForm(msg.projects)

	case environmentsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		if len(msg.envs) == 0 {
			return m, m.afterEnvironment()
		}
		return m, m.buildEnvironmentForm(msg.envs)

	case kvCreateDoneMsg:
		m.result = msg.kv
		m.err = msg.err
		m.step = stepDone
		return m, tea.Quit
	}

	// Forward everything else to the active form. huh handles arrow keys,
	// enter, validation, theming, etc.
	if m.form != nil {
		next, cmd := m.form.Update(msg)
		if f, ok := next.(*huh.Form); ok {
			m.form = f
		}
		switch m.form.State {
		case huh.StateAborted:
			m.canceled = true
			return m, tea.Quit
		case huh.StateCompleted:
			return m.onFormCompleted()
		}
		return m, cmd
	}
	return m, nil
}

func (m *KeyValueCreateModel) View() string {
	var b strings.Builder
	if len(m.completed) > 0 {
		b.WriteString("\n")
		for _, c := range m.completed {
			b.WriteString(renderCompletedKVStep(c))
			b.WriteString("\n")
		}
	}
	switch m.step {
	case stepInit, stepDone:
		// Completed lines above are the final transcript; no live region.
	case stepLoadingWorkspaces, stepLoadingProjects, stepLoadingEnvironments, stepCreating:
		b.WriteString(fmt.Sprintf("\n%s %s...\n", m.spinner.View(), m.loadingMsg))
	default:
		if m.form != nil {
			b.WriteString("\n")
			b.WriteString(m.form.View())
		}
	}
	return b.String()
}

// --- step orchestration ---

// advance figures out what the next step should be given current input and
// either kicks off async work or constructs the next form.
func (m *KeyValueCreateModel) advance() tea.Cmd {
	if m.input.WorkspaceIDOrName == "" {
		m.step = stepLoadingWorkspaces
		m.loadingMsg = "Loading workspaces"
		return tea.Batch(m.spinner.Tick, m.loadWorkspacesCmd())
	}
	if !validate.IsWorkspaceID(m.input.WorkspaceIDOrName) {
		m.step = stepLoadingWorkspaces
		m.loadingMsg = "Resolving workspace"
		return tea.Batch(m.spinner.Tick, m.resolveWorkspaceCmd(m.input.WorkspaceIDOrName))
	}
	return m.afterWorkspace()
}

func (m *KeyValueCreateModel) afterWorkspace() tea.Cmd {
	if m.input.EnvironmentIDOrName != nil {
		return m.afterEnvironment()
	}
	if m.input.ProjectIDOrName != nil {
		m.step = stepLoadingEnvironments
		m.loadingMsg = "Loading environments"
		return tea.Batch(m.spinner.Tick, m.loadEnvsCmd(*m.input.ProjectIDOrName))
	}
	m.step = stepLoadingProjects
	m.loadingMsg = "Loading projects"
	return tea.Batch(m.spinner.Tick, m.loadProjectsCmd())
}

func (m *KeyValueCreateModel) afterEnvironment() tea.Cmd {
	if m.input.Name == "" {
		return m.buildNameForm()
	}
	return m.afterName()
}

func (m *KeyValueCreateModel) afterName() tea.Cmd {
	if m.input.Plan == "" {
		return m.buildPlanForm()
	}
	return m.afterPlan()
}

func (m *KeyValueCreateModel) afterPlan() tea.Cmd {
	if m.input.Region == nil {
		return m.buildRegionForm()
	}
	return m.afterRegion()
}

func (m *KeyValueCreateModel) afterRegion() tea.Cmd {
	if m.input.MaxmemoryPolicy == nil {
		return m.buildMemoryPolicyForm()
	}
	return m.afterMemoryPolicy()
}

func (m *KeyValueCreateModel) afterMemoryPolicy() tea.Cmd {
	return m.gotoConfirm()
}

func (m *KeyValueCreateModel) gotoConfirm() tea.Cmd {
	m.step = stepConfirm
	m.confirmValue = false
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Create this Key Value store?").
			Value(&m.confirmValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

// onFormCompleted reads the form's bound value, records a completed-step
// breadcrumb, and transitions to the next step.
func (m *KeyValueCreateModel) onFormCompleted() (tea.Model, tea.Cmd) {
	step := m.step
	m.form = nil

	switch step {
	case stepWorkspace:
		m.input.WorkspaceIDOrName = m.selectValue
		m.completed = append(m.completed, completedStep{"Workspace", m.labelByValue[m.selectValue]})
		return m, m.afterWorkspace()

	case stepName:
		val := strings.TrimSpace(m.nameValue)
		if val == "" {
			val = m.namePlaceholder
		}
		m.input.Name = val
		m.completed = append(m.completed, completedStep{"Name", val})
		return m, m.afterName()

	case stepPlan:
		m.input.Plan = kvtypes.Plan(m.selectValue)
		m.completed = append(m.completed, completedStep{"Plan", m.labelByValue[m.selectValue]})
		return m, m.afterPlan()

	case stepRegion:
		v := m.selectValue
		m.input.Region = &v
		m.completed = append(m.completed, completedStep{"Region", m.labelByValue[v]})
		return m, m.afterRegion()

	case stepMemoryPolicy:
		p := kvtypes.MaxmemoryPolicy(m.selectValue)
		m.input.MaxmemoryPolicy = &p
		m.completed = append(m.completed, completedStep{"Memory policy", m.selectValue})
		return m, m.afterMemoryPolicy()

	case stepProject:
		if m.selectValue == "__none__" {
			m.completed = append(m.completed, completedStep{"Project", "(none)"})
			return m, m.afterEnvironment()
		}
		pid := m.selectValue
		m.input.ProjectIDOrName = &pid
		m.completed = append(m.completed, completedStep{"Project", m.labelByValue[pid]})
		m.step = stepLoadingEnvironments
		m.loadingMsg = "Loading environments"
		return m, tea.Batch(m.spinner.Tick, m.loadEnvsCmd(pid))

	case stepEnvironment:
		eid := m.selectValue
		m.input.EnvironmentIDOrName = &eid
		m.completed = append(m.completed, completedStep{"Environment", m.labelByValue[eid]})
		return m, m.afterEnvironment()

	case stepConfirm:
		if !m.confirmValue {
			m.canceled = true
			return m, tea.Quit
		}
		m.step = stepCreating
		m.loadingMsg = "Creating Key Value store"
		return m, tea.Batch(m.spinner.Tick, m.createCmd())
	}
	return m, nil
}

// --- per-step form constructors ---

func (m *KeyValueCreateModel) buildWorkspaceForm(owners []*client.Owner) tea.Cmd {
	active, _ := config.WorkspaceID()
	options := make([]huh.Option[string], 0, len(owners))
	labels := make(map[string]string, len(owners))
	var defaultVal string
	for _, o := range owners {
		label := o.Name
		if o.Id == active {
			label += " (active)"
			defaultVal = o.Id
		}
		labels[o.Id] = label
		options = append(options, huh.NewOption(label, o.Id))
	}
	if defaultVal == "" && len(owners) > 0 {
		defaultVal = owners[0].Id
	}
	m.selectValue = defaultVal
	m.labelByValue = labels
	m.step = stepWorkspace
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Workspace").
			Options(options...).
			Value(&m.selectValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

func (m *KeyValueCreateModel) buildNameForm() tea.Cmd {
	m.step = stepName
	m.namePlaceholder = petname.Generate(2, "-")
	m.nameValue = ""
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Description("Human-readable label for this Key Value instance (e.g. my-app-cache).").
			Placeholder(m.namePlaceholder).
			Value(&m.nameValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

func (m *KeyValueCreateModel) buildPlanForm() tea.Cmd {
	m.step = stepPlan
	m.selectValue = string(kvtypes.PlanFree)
	m.labelByValue = map[string]string{
		string(client.KeyValuePlanFree):     "Free",
		string(client.KeyValuePlanStarter):  "Starter",
		string(client.KeyValuePlanStandard): "Standard",
		string(client.KeyValuePlanPro):      "Pro",
		string(client.KeyValuePlanProPlus):  "Pro Plus",
	}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Plan").
			Options(
				huh.NewOption("Free", string(client.KeyValuePlanFree)),
				huh.NewOption("Starter", string(client.KeyValuePlanStarter)),
				huh.NewOption("Standard", string(client.KeyValuePlanStandard)),
				huh.NewOption("Pro", string(client.KeyValuePlanPro)),
				huh.NewOption("Pro Plus", string(client.KeyValuePlanProPlus)),
			).
			Value(&m.selectValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

func (m *KeyValueCreateModel) buildRegionForm() tea.Cmd {
	m.step = stepRegion
	m.selectValue = string(types.RegionOregon)
	m.labelByValue = map[string]string{
		string(types.RegionOregon):    "Oregon (US West)",
		string(types.RegionOhio):      "Ohio (US East)",
		string(types.RegionVirginia):  "Virginia (US East)",
		string(types.RegionFrankfurt): "Frankfurt (EU)",
		string(types.RegionSingapore): "Singapore (Asia)",
	}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Region").
			Options(
				huh.NewOption("Oregon (US West)", string(types.RegionOregon)),
				huh.NewOption("Ohio (US East)", string(types.RegionOhio)),
				huh.NewOption("Virginia (US East)", string(types.RegionVirginia)),
				huh.NewOption("Frankfurt (EU)", string(types.RegionFrankfurt)),
				huh.NewOption("Singapore (Asia)", string(types.RegionSingapore)),
			).
			Value(&m.selectValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

func (m *KeyValueCreateModel) buildMemoryPolicyForm() tea.Cmd {
	m.step = stepMemoryPolicy
	m.selectValue = string(client.AllkeysLru)
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Memory Policy").
			Description("Controls what happens when the instance runs out of memory.").
			Options(
				huh.NewOption("allkeys_lru — evict any key (LRU); recommended for caching", string(client.AllkeysLru)),
				huh.NewOption("noeviction — return error when full; recommended for job queues", string(client.Noeviction)),
				huh.NewOption("allkeys_lfu — evict any key (LFU)", string(client.AllkeysLfu)),
				huh.NewOption("allkeys_random — evict random key", string(client.AllkeysRandom)),
				huh.NewOption("volatile_lru — evict expiring key (LRU)", string(client.VolatileLru)),
				huh.NewOption("volatile_lfu — evict expiring key (LFU)", string(client.VolatileLfu)),
				huh.NewOption("volatile_random — evict random expiring key", string(client.VolatileRandom)),
				huh.NewOption("volatile_ttl — evict soonest-expiring key", string(client.VolatileTtl)),
			).
			Value(&m.selectValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

func (m *KeyValueCreateModel) buildProjectForm(projects []*client.Project) tea.Cmd {
	const noProject = "__none__"
	options := []huh.Option[string]{huh.NewOption("(no project/environment)", noProject)}
	labels := map[string]string{noProject: "(none)"}
	for _, p := range projects {
		labels[p.Id] = p.Name
		options = append(options, huh.NewOption(p.Name, p.Id))
	}
	m.selectValue = noProject
	m.labelByValue = labels
	m.step = stepProject
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Project").
			Description("Optionally associate this Key Value store with a project environment.").
			Options(options...).
			Value(&m.selectValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

func (m *KeyValueCreateModel) buildEnvironmentForm(envs []*client.Environment) tea.Cmd {
	options := make([]huh.Option[string], 0, len(envs))
	labels := make(map[string]string, len(envs))
	for _, e := range envs {
		labels[e.Id] = e.Name
		options = append(options, huh.NewOption(e.Name, e.Id))
	}
	if len(envs) > 0 {
		m.selectValue = envs[0].Id
	}
	m.labelByValue = labels
	m.step = stepEnvironment
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Environment").
			Options(options...).
			Value(&m.selectValue),
	)).WithShowHelp(false)
	return m.form.Init()
}

// --- async commands ---

func (m KeyValueCreateModel) loadWorkspacesCmd() tea.Cmd {
	ctx := m.ctx
	repo := m.repos.owners
	return func() tea.Msg {
		os, err := repo.ListOwners(ctx, owner.ListInput{})
		return workspacesLoadedMsg{owners: os, err: err}
	}
}

func (m KeyValueCreateModel) resolveWorkspaceCmd(name string) tea.Cmd {
	ctx := m.ctx
	r := m.repos.resolver
	return func() tea.Msg {
		id, err := r.ResolveWorkspaceID(ctx, name)
		return workspaceResolvedMsg{workspaceID: id, err: err}
	}
}

func (m KeyValueCreateModel) loadProjectsCmd() tea.Cmd {
	ctx := m.ctx
	repo := m.repos.projects
	workspaceID := m.input.WorkspaceIDOrName
	return func() tea.Msg {
		ps, err := repo.ListProjectsForWorkspace(ctx, workspaceID)
		return projectsLoadedMsg{projects: ps, err: err}
	}
}

func (m KeyValueCreateModel) loadEnvsCmd(projectID string) tea.Cmd {
	ctx := m.ctx
	repo := m.repos.envs
	return func() tea.Msg {
		es, err := repo.ListEnvironments(ctx, &client.ListEnvironmentsParams{
			ProjectId: []string{projectID},
		})
		return environmentsLoadedMsg{envs: es, err: err}
	}
}

func (m KeyValueCreateModel) createCmd() tea.Cmd {
	ctx := m.ctx
	in := m.input
	return func() tea.Msg {
		kv, err := keyvalue.Create(ctx, in)
		return kvCreateDoneMsg{kv: kv, err: err}
	}
}

// --- helpers ---

func renderCompletedKVStep(c completedStep) string {
	check := lipgloss.NewStyle().Foreground(renderstyle.ColorOK).Render("✓")
	return fmt.Sprintf("  %s %s: %s", check, c.label, c.value)
}
