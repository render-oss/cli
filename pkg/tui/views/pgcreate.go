package views

// PostgresCreateModel is an inline interactive wizard for `render ea pg create`.
// The outer tea.Model owns step orchestration via a declarative step pipeline;
// each individual step is rendered by a huh.Form composed as a child tea.Model.
//
// Unlike kv create (where step sequencing is implied by chains of afterX()
// calls), this wizard defines the full sequence as a package-level slice
// (pgCreateSteps). advanceToStep() walks the slice, skipping conditional steps,
// and calls each step's load/buildForm/commit functions generically.
//
// The program runs inline (no alt-screen) to preserve shell scrollback.

import (
	"context"
	"fmt"
	"strconv"
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
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/project"
	rstrings "github.com/render-oss/cli/pkg/strings"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/types"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

// pgCreateWizardStep describes one step in the interactive pg create wizard.
type pgCreateWizardStep struct {
	// label is shown in breadcrumbs and loading messages.
	label string
	// condition, if non-nil, is evaluated before entering the step; the step is
	// skipped when it returns false. Evaluated after any earlier-step commit has run,
	// so it can reference collected wizard values (e.g. m.draft.projectID, m.draft.plan).
	condition func(*PostgresCreateModel) bool
	// load, if non-nil, fires an async command before buildForm. The model enters
	// a spinner/loading state. The returned command must send a message that is
	// handled in Update(), where results are stored on m.loaded, and then
	// buildForm is called. Nil for steps that need no async data.
	load func(*PostgresCreateModel) tea.Cmd
	// buildForm builds the huh.Form for this step. Called directly by
	// advanceToStep when load is nil, or by async message handlers after load completes.
	buildForm func(*PostgresCreateModel) *huh.Form
	// commit interprets the completed form and returns a pgStepCommit.
	// It must not mutate the model or breadcrumbs directly; the orchestrator does that.
	commit func(*PostgresCreateModel) pgStepCommit
}

// pgStepCommit is the result of a completed wizard step.
// The orchestrator (onFormCompleted) applies it to the model.
type pgStepCommit struct {
	// displayValue is shown in the breadcrumb for this step. Empty means no breadcrumb.
	displayValue string
	// apply writes the step's domain value(s) into the draft. Nil for the Confirm step.
	apply func(*pgCreateDraft)
	// canceled signals that the user declined to proceed (Confirm step answered No).
	canceled bool
}

// pgCreateDraft holds the domain values collected by the wizard.
// It is populated incrementally by each step's commit, and consumed by createCmd.
type pgCreateDraft struct {
	workspaceID     string
	projectID       *string
	environmentID   *string
	name            string
	plan            string
	version         int
	region          string
	ha              bool
	diskAutoscaling bool
}

// pgCreateLoadedData holds data fetched asynchronously from the server.
type pgCreateLoadedData struct {
	owners   []*client.Owner
	projects []*client.Project
	envs     []*client.Environment
}

// pgSelectState holds the transient value and display labels for an active select form.
type pgSelectState struct {
	value  string
	labels map[string]string
}

func (s pgSelectState) label() string {
	return s.labels[s.value]
}

type pgSelectOption struct {
	value string
	label string
}

func pgSelectOptions(items []pgSelectOption) ([]huh.Option[string], map[string]string) {
	options := make([]huh.Option[string], 0, len(items))
	labels := make(map[string]string, len(items))
	for _, item := range items {
		options = append(options, huh.NewOption(item.label, item.value))
		labels[item.value] = item.label
	}
	return options, labels
}

// PostgresCreateRepos holds the data-access dependencies for the pg create wizard.
// Populate from *dependencies.Dependencies in both production and tests:
//
//	repos := views.PostgresCreateRepos{
//	    Owners:   deps.OwnerRepo(),
//	    Projects: deps.ProjectRepo(),
//	    Envs:     deps.EnvironmentRepo(),
//	    Postgres: deps.PostgresRepo(),
//	}
type PostgresCreateRepos struct {
	Owners   *owner.Repo
	Projects *project.Repo
	Envs     *environment.Repo
	Postgres *postgres.Repo
}

// PostgresCreateModel is the Bubbletea model for the pg create wizard.
type PostgresCreateModel struct {
	ctx   context.Context
	repos PostgresCreateRepos
	// flagInput carries flag-only values (ip-allow-list, database-name, etc.)
	flagInput pgtypes.CreatePostgresInput

	// draft accumulates the pg config values chosen by the user.
	draft pgCreateDraft
	// loaded holds data fetched from the server for the current step's form.
	loaded pgCreateLoadedData

	currentStep int
	breadcrumbs []pgBreadcrumb
	form        *huh.Form
	spinner     spinner.Model
	loadingMsg  string

	// Form-bound values — huh writes into these directly.
	selectState     pgSelectState
	nameValue       string
	namePlaceholder string
	confirmValue    bool

	// Outcomes.
	canceled bool
	err      error
	result   *client.PostgresDetail
}

// pgBreadcrumb is a completed-step summary line shown above the active prompt.
type pgBreadcrumb struct {
	label string
	value string
}

// --- async message types ---

type pgOwnersLoadedMsg struct {
	owners []*client.Owner
	err    error
}
type pgProjectsLoadedMsg struct {
	projects []*client.Project
	err      error
}
type pgEnvsLoadedMsg struct {
	envs []*client.Environment
	err  error
}
type pgCreateDoneMsg struct {
	pg  *client.PostgresDetail
	err error
}

// --- declarative step pipeline ---

const pgNoProjectSentinel = "__none__"

// pgCreateSteps is the ordered, authoritative list of wizard steps.
// To add, remove, or reorder a step, edit this slice only —
// Init/Update/View and the orchestration helpers are generic.
var pgCreateSteps = []pgCreateWizardStep{
	{
		label:     "Workspace",
		load:      func(m *PostgresCreateModel) tea.Cmd { return m.loadWorkspacesCmd() },
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildWorkspaceForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			id := m.selectState.value
			return pgStepCommit{
				displayValue: m.selectState.label(),
				apply:        func(d *pgCreateDraft) { d.workspaceID = id },
			}
		},
	},
	{
		label:     "Project",
		load:      func(m *PostgresCreateModel) tea.Cmd { return m.loadProjectsCmd() },
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildProjectForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			if m.selectState.value == pgNoProjectSentinel {
				return pgStepCommit{displayValue: "(none)"}
				// draft.projectID stays nil → environment step will be skipped
			}
			pid := m.selectState.value
			return pgStepCommit{
				displayValue: m.selectState.label(),
				apply:        func(d *pgCreateDraft) { d.projectID = &pid },
			}
		},
	},
	{
		label:     "Environment",
		condition: func(m *PostgresCreateModel) bool { return m.draft.projectID != nil },
		load:      func(m *PostgresCreateModel) tea.Cmd { return m.loadEnvsCmd(*m.draft.projectID) },
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildEnvironmentForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			eid := m.selectState.value
			return pgStepCommit{
				displayValue: m.selectState.label(),
				apply:        func(d *pgCreateDraft) { d.environmentID = &eid },
			}
		},
	},
	{
		label:     "Name",
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildNameForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			val := strings.TrimSpace(m.nameValue)
			if val == "" {
				val = m.namePlaceholder
			}
			return pgStepCommit{
				displayValue: val,
				apply:        func(d *pgCreateDraft) { d.name = val },
			}
		},
	},
	{
		label:     "Plan",
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildPlanForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			plan := m.selectState.value
			return pgStepCommit{
				displayValue: pgPlanLabel(plan),
				apply:        func(d *pgCreateDraft) { d.plan = plan },
			}
		},
	},
	{
		label:     "Version",
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildVersionForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			v, _ := strconv.Atoi(m.selectState.value)
			return pgStepCommit{
				displayValue: "PostgreSQL " + m.selectState.value,
				apply:        func(d *pgCreateDraft) { d.version = v },
			}
		},
	},
	{
		label:     "Region",
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildRegionForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			region := m.selectState.value
			return pgStepCommit{
				displayValue: m.selectState.label(),
				apply:        func(d *pgCreateDraft) { d.region = region },
			}
		},
	},
	{
		label:     "High Availability",
		condition: func(m *PostgresCreateModel) bool { return isHAEligiblePlan(m.draft.plan) },
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildHAForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			ha := m.confirmValue
			label := "No"
			if ha {
				label = "Yes"
			}
			return pgStepCommit{
				displayValue: label,
				apply:        func(d *pgCreateDraft) { d.ha = ha },
			}
		},
	},
	{
		label:     "Disk Autoscaling",
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildDiskAutoscalingForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			auto := m.confirmValue
			label := "No"
			if auto {
				label = "Yes"
			}
			return pgStepCommit{
				displayValue: label,
				apply:        func(d *pgCreateDraft) { d.diskAutoscaling = auto },
			}
		},
	},
	{
		label:     "Confirm",
		buildForm: func(m *PostgresCreateModel) *huh.Form { return m.buildConfirmForm() },
		commit: func(m *PostgresCreateModel) pgStepCommit {
			return pgStepCommit{canceled: !m.confirmValue}
		},
	},
}

// --- constructors ---

// NewPostgresCreateModel constructs the wizard model. Used both by RunPostgresCreate
// and directly in tests (populate repos from dependencies.New(c)).
func NewPostgresCreateModel(ctx context.Context, repos PostgresCreateRepos, flagInput pgtypes.CreatePostgresInput) *PostgresCreateModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &PostgresCreateModel{
		ctx:       ctx,
		repos:     repos,
		flagInput: flagInput,
		spinner:   sp,
	}
}

// RunPostgresCreate is the entry point from the cmd layer.
// It prints a header, runs the wizard inline (no alt-screen), and returns the
// created PostgresDetail, or nil if the user canceled.
func RunPostgresCreate(cmd *cobra.Command, repos PostgresCreateRepos, flagInput pgtypes.CreatePostgresInput) (*client.PostgresDetail, error) {
	m := NewPostgresCreateModel(cmd.Context(), repos, flagInput)

	command.Println(cmd, "")
	command.Println(cmd, "%s", renderstyle.Bold("Creating Postgres instance"))

	p := tea.NewProgram(m, tea.WithContext(cmd.Context()), tea.WithOutput(cmd.OutOrStdout()))
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := finalModel.(*PostgresCreateModel)
	if fm.err != nil {
		return nil, fm.err
	}
	if fm.canceled {
		return nil, nil
	}
	return fm.result, nil
}

// --- tea.Model interface ---

func (m *PostgresCreateModel) Init() tea.Cmd {
	_, cmd := m.advanceToStep(0)
	return cmd
}

func (m *PostgresCreateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global exit.
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyCtrlC {
		m.canceled = true
		return m, tea.Quit
	}

	// Spinner tick.
	if _, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Async results — store data then let the current step build its form.
	switch msg := msg.(type) {
	case pgOwnersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.loaded.owners = msg.owners
		return m, m.startForm(pgCreateSteps[m.currentStep].buildForm(m))

	case pgProjectsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.loaded.projects = msg.projects
		if len(msg.projects) == 0 {
			// No projects — skip project and (implicitly) environment steps.
			return m.advanceToStep(m.currentStep + 1)
		}
		return m, m.startForm(pgCreateSteps[m.currentStep].buildForm(m))

	case pgEnvsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.loaded.envs = msg.envs
		if len(msg.envs) == 0 {
			// No environments for this project — skip the environment step.
			return m.advanceToStep(m.currentStep + 1)
		}
		return m, m.startForm(pgCreateSteps[m.currentStep].buildForm(m))

	case pgCreateDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.result = msg.pg
		m.form = nil
		m.loadingMsg = ""
		return m, tea.Quit
	}

	// Forward all other messages to the active form.
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

func (m *PostgresCreateModel) View() string {
	var b strings.Builder
	if len(m.breadcrumbs) > 0 {
		b.WriteString("\n")
		for _, c := range m.breadcrumbs {
			b.WriteString(renderCompletedPGStep(c))
			b.WriteString("\n")
		}
	}
	if m.result != nil {
		b.WriteString(renderPostgresCreateSuccess(m.result))
	} else if m.canceled {
		b.WriteString("\nCanceled.\n")
	} else if m.form != nil {
		b.WriteString("\n")
		b.WriteString(m.form.View())
	} else if m.loadingMsg != "" {
		fmt.Fprintf(&b, "\n%s %s...\n", m.spinner.View(), m.loadingMsg)
	}
	return b.String()
}

// --- step orchestration ---

// advanceToStep advances to step i, skipping any whose condition returns false.
// When all steps are exhausted it fires the create command.
func (m *PostgresCreateModel) advanceToStep(i int) (tea.Model, tea.Cmd) {
	if m.canceled {
		return m, tea.Quit
	}
	for i < len(pgCreateSteps) {
		s := pgCreateSteps[i]
		if s.condition == nil || s.condition(m) {
			break
		}
		i++
	}
	if i >= len(pgCreateSteps) {
		m.loadingMsg = "Creating Postgres instance"
		return m, tea.Batch(m.spinner.Tick, m.createCmd())
	}
	m.currentStep = i
	s := pgCreateSteps[i]
	if s.load != nil {
		m.loadingMsg = "Loading " + strings.ToLower(s.label) + "s"
		return m, tea.Batch(m.spinner.Tick, s.load(m))
	}
	return m, m.startForm(s.buildForm(m))
}

func (m *PostgresCreateModel) startForm(form *huh.Form) tea.Cmd {
	m.form = form
	return m.form.Init()
}

// onFormCompleted applies the current step's commit result and advances.
func (m *PostgresCreateModel) onFormCompleted() (tea.Model, tea.Cmd) {
	step := pgCreateSteps[m.currentStep]
	commit := step.commit(m)

	if commit.canceled {
		m.canceled = true
		m.form = nil
		return m, tea.Quit
	}
	if commit.apply != nil {
		commit.apply(&m.draft)
	}
	if commit.displayValue != "" {
		m.breadcrumbs = append(m.breadcrumbs, pgBreadcrumb{
			label: step.label,
			value: commit.displayValue,
		})
	}

	m.form = nil
	return m.advanceToStep(m.currentStep + 1)
}

// --- async commands ---

func (m *PostgresCreateModel) loadWorkspacesCmd() tea.Cmd {
	ctx := m.ctx
	repo := m.repos.Owners
	return func() tea.Msg {
		owners, err := repo.ListOwners(ctx, owner.ListInput{})
		return pgOwnersLoadedMsg{owners: owners, err: err}
	}
}

func (m *PostgresCreateModel) loadProjectsCmd() tea.Cmd {
	ctx := m.ctx
	repo := m.repos.Projects
	workspaceID := m.draft.workspaceID
	return func() tea.Msg {
		projects, err := repo.ListProjectsForWorkspace(ctx, workspaceID)
		return pgProjectsLoadedMsg{projects: projects, err: err}
	}
}

func (m *PostgresCreateModel) loadEnvsCmd(projectID string) tea.Cmd {
	ctx := m.ctx
	repo := m.repos.Envs
	return func() tea.Msg {
		envs, err := repo.ListEnvironments(ctx, &client.ListEnvironmentsParams{
			ProjectId: []string{projectID},
		})
		return pgEnvsLoadedMsg{envs: envs, err: err}
	}
}

func (m *PostgresCreateModel) createCmd() tea.Cmd {
	ctx := m.ctx
	reqInput := m.createRequestInput()
	repo := m.repos.Postgres
	return func() tea.Msg {
		body, err := postgres.BuildCreateRequest(reqInput)
		if err != nil {
			return pgCreateDoneMsg{err: err}
		}
		pg, err := repo.CreatePostgres(ctx, body)
		return pgCreateDoneMsg{pg: pg, err: err}
	}
}

func (m *PostgresCreateModel) createRequestInput() postgres.CreateRequestInput {
	haVal := m.draft.ha
	diskAutoVal := m.draft.diskAutoscaling
	return postgres.CreateRequestInput{
		Name:             m.draft.name,
		OwnerID:          m.draft.workspaceID,
		Plan:             m.draft.plan,
		Version:          m.draft.version,
		Region:           &m.draft.region,
		EnvironmentID:    m.draft.environmentID,
		HighAvailability: &haVal,
		DiskAutoscaling:  &diskAutoVal,
		// Flag-only fields carried through from the original parsed input.
		DiskSizeGB:    m.flagInput.DiskSizeGB,
		DatabaseName:  m.flagInput.DatabaseName,
		DatabaseUser:  m.flagInput.DatabaseUser,
		DatadogAPIKey: m.flagInput.DatadogAPIKey,
		DatadogSite:   m.flagInput.DatadogSite,
		IPAllowList:   m.flagInput.IPAllowList,
		ReadReplicas:  m.flagInput.ReadReplicas,
	}
}

// --- per-step form builders ---

func (m *PostgresCreateModel) buildWorkspaceForm() *huh.Form {
	active, _ := config.WorkspaceID()
	options := make([]huh.Option[string], 0, len(m.loaded.owners))
	labels := make(map[string]string, len(m.loaded.owners))
	var defaultVal string
	for _, o := range m.loaded.owners {
		label := o.Name
		if o.Id == active {
			label += " (active)"
			defaultVal = o.Id
		}
		labels[o.Id] = label
		options = append(options, huh.NewOption(label, o.Id))
	}
	if defaultVal == "" && len(m.loaded.owners) > 0 {
		defaultVal = m.loaded.owners[0].Id
	}
	m.selectState = pgSelectState{value: defaultVal, labels: labels}

	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Workspace").
			Options(options...).
			Value(&m.selectState.value),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildProjectForm() *huh.Form {
	options := []huh.Option[string]{huh.NewOption("(no project/environment)", pgNoProjectSentinel)}
	labels := map[string]string{pgNoProjectSentinel: "(none)"}
	for _, p := range m.loaded.projects {
		labels[p.Id] = p.Name
		options = append(options, huh.NewOption(p.Name, p.Id))
	}
	m.selectState = pgSelectState{value: pgNoProjectSentinel, labels: labels}

	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Project").
			Description("Optionally associate this database with a project environment.").
			Options(options...).
			Value(&m.selectState.value),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildEnvironmentForm() *huh.Form {
	options := make([]huh.Option[string], 0, len(m.loaded.envs))
	labels := make(map[string]string, len(m.loaded.envs))
	for _, e := range m.loaded.envs {
		labels[e.Id] = e.Name
		options = append(options, huh.NewOption(e.Name, e.Id))
	}
	if len(m.loaded.envs) > 0 {
		m.selectState = pgSelectState{value: m.loaded.envs[0].Id, labels: labels}
	} else {
		m.selectState = pgSelectState{labels: labels}
	}

	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Environment").
			Options(options...).
			Value(&m.selectState.value),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildNameForm() *huh.Form {
	m.namePlaceholder = petname.Generate(2, "-")
	m.nameValue = ""
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Description("Human-readable label for this Postgres instance (e.g. my-app-db).").
			Placeholder(m.namePlaceholder).
			Value(&m.nameValue),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildPlanForm() *huh.Form {
	items := make([]pgSelectOption, 0, len(postgres.ModernPlans))
	for _, p := range postgres.ModernPlans {
		items = append(items, pgSelectOption{value: p, label: pgPlanLabel(p)})
	}
	options, labels := pgSelectOptions(items)
	m.selectState = pgSelectState{value: "free", labels: labels}
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Plan").
			Options(options...).
			Value(&m.selectState.value),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildVersionForm() *huh.Form {
	m.selectState = pgSelectState{value: strconv.Itoa(postgres.DefaultPostgresVersion)}
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Postgres Version").
			Options(
				huh.NewOption("PostgreSQL 18", "18"),
				huh.NewOption("PostgreSQL 17", "17"),
				huh.NewOption("PostgreSQL 16", "16"),
				huh.NewOption("PostgreSQL 15", "15"),
				huh.NewOption("PostgreSQL 14", "14"),
			).
			Value(&m.selectState.value),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildRegionForm() *huh.Form {
	options, labels := pgSelectOptions([]pgSelectOption{
		{value: string(types.RegionOregon), label: "Oregon (US West)"},
		{value: string(types.RegionOhio), label: "Ohio (US East)"},
		{value: string(types.RegionVirginia), label: "Virginia (US East)"},
		{value: string(types.RegionFrankfurt), label: "Frankfurt (EU)"},
		{value: string(types.RegionSingapore), label: "Singapore (Asia)"},
	})
	m.selectState = pgSelectState{value: string(types.RegionOregon), labels: labels}
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Region").
			Options(options...).
			Value(&m.selectState.value),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildHAForm() *huh.Form {
	m.confirmValue = false
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Enable High Availability?").
			Description("Deploys a standby replica in a separate availability zone. Requires a Pro plan or higher.").
			Value(&m.confirmValue),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildDiskAutoscalingForm() *huh.Form {
	m.confirmValue = false
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Enable Disk Autoscaling?").
			Description("Automatically expands disk storage when usage exceeds 90%.").
			Value(&m.confirmValue),
	)).WithShowHelp(false)
}

func (m *PostgresCreateModel) buildConfirmForm() *huh.Form {
	m.confirmValue = false
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Create this Postgres instance?").
			Value(&m.confirmValue),
	)).WithShowHelp(false)
}

// --- helpers ---

// isHAEligiblePlan reports whether the given plan supports high availability.
// Pro and Accelerated plans are eligible; Free and Basic plans are not.
// This mirrors the server-side check in pkg/userdb/plan.go (NoHA flag).
func isHAEligiblePlan(plan string) bool {
	return strings.HasPrefix(plan, "pro_") || strings.HasPrefix(plan, "accelerated_")
}

// pgPlanLabel converts an internal plan identifier to a user-friendly label.
// "free" → "Free", "pro_4gb" → "Pro 4GB", "accelerated_16gb" → "Accelerated 16GB".
func pgPlanLabel(plan string) string {
	parts := strings.SplitN(plan, "_", 2)
	if len(parts) == 2 {
		tier := strings.ToUpper(parts[0][:1]) + parts[0][1:]
		return tier + " " + strings.ToUpper(parts[1])
	}
	return strings.ToUpper(plan[:1]) + plan[1:]
}

func renderCompletedPGStep(c pgBreadcrumb) string {
	check := lipgloss.NewStyle().Foreground(renderstyle.ColorOK).Render("✓")
	return fmt.Sprintf("  %s %s: %s", check, c.label, c.value)
}

func renderPostgresCreateSuccess(pg *client.PostgresDetail) string {
	success := lipgloss.NewStyle().
		Foreground(renderstyle.ColorOK).
		Bold(true).
		Render("Success!")

	return fmt.Sprintf(
		"\n%s\n\nDatabase %s was created.\n\nRun `render ea pg get %s` to check if it's ready yet.\n",
		success,
		rstrings.ResourceLabel(pg.Name, pg.Id),
		pg.Name,
	)
}
