package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallInput holds the parameters for a skills install operation.
type InstallInput struct {
	// ToolFilter filters tools by name (optional, empty means all detected tools).
	ToolFilter string
	// SkillFilter specifies which skills to install by name or dir (optional, empty means all).
	SkillFilter []string
	// DryRun if true, shows what would be installed without making changes.
	DryRun bool
	// PreSelectedTools allows passing already-selected tools (used by TUI).
	PreSelectedTools []Tool
	// PreSelectedSkillNames allows passing already-selected skill names (used by TUI).
	PreSelectedSkillNames []string
	// Scope specifies where to install skills (user or project). Defaults to user.
	Scope Scope
	// RepoRoot is the git repository root (required for project scope).
	RepoRoot string
}

// Install performs a skills installation and returns the result.
// It handles the full flow: detect tools, clone repo, filter skills, install, save state.
func Install(input InstallInput) (*InstallResult, error) {
	// Default to user scope if not specified
	scope := input.Scope
	if scope == "" {
		scope = ScopeUser
	}

	// For project scope, we need a repo root
	repoRoot := input.RepoRoot
	if scope == ScopeProject {
		if repoRoot == "" {
			var err error
			repoRoot, err = GetRepoRoot()
			if err != nil {
				return nil, fmt.Errorf("project scope requires a git repository: %w", err)
			}
		}
	}

	// Determine which tools to use
	var selectedTools []Tool
	if len(input.PreSelectedTools) > 0 {
		selectedTools = input.PreSelectedTools
	} else {
		allTools, err := DetectTools()
		if err != nil {
			return nil, fmt.Errorf("failed to detect tools: %w", err)
		}
		if len(allTools) == 0 {
			return nil, fmt.Errorf("no supported AI coding tools detected")
		}

		if input.ToolFilter != "" {
			selectedTools = FilterTools(allTools, input.ToolFilter)
			if len(selectedTools) == 0 {
				return nil, fmt.Errorf("no installed tool matching %q found", input.ToolFilter)
			}
		} else {
			selectedTools = allTools
		}
	}

	// Clone the skills repo
	tmpDir, err := os.MkdirTemp("", "render-skills-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := CloneSkillsRepo(tmpDir); err != nil {
		return nil, err
	}

	// Read available skills
	available := ReadSkillsFromRepo(tmpDir)
	if len(available) == 0 {
		return nil, fmt.Errorf("no skills found in the repository")
	}

	// Determine which skills to install
	var selectedSkillNames []string
	if len(input.PreSelectedSkillNames) > 0 {
		selectedSkillNames = input.PreSelectedSkillNames
	} else if len(input.SkillFilter) > 0 {
		// Resolve filter values to directory names
		nameToDir := make(map[string]string, len(available)*2)
		for _, s := range available {
			nameToDir[s.Name] = s.DirName
			nameToDir[s.DirName] = s.DirName
		}
		var unmatched []string
		for _, name := range input.SkillFilter {
			if dirName, ok := nameToDir[name]; ok {
				selectedSkillNames = append(selectedSkillNames, dirName)
			} else {
				unmatched = append(unmatched, name)
			}
		}
		if len(selectedSkillNames) == 0 {
			return nil, fmt.Errorf("no skills matching %q found in the repository", strings.Join(unmatched, ", "))
		}
		// Note: unmatched skills are silently ignored; caller can warn if needed
	}
	// If no filter and no pre-selection, all skills will be installed (selectedSkillNames stays nil)

	// Build result with selected skills info
	var selectedSkills []SkillInfo
	if len(selectedSkillNames) > 0 {
		selectedSet := make(map[string]bool, len(selectedSkillNames))
		for _, n := range selectedSkillNames {
			selectedSet[n] = true
		}
		for _, s := range available {
			if selectedSet[s.DirName] {
				selectedSkills = append(selectedSkills, s)
			}
		}
	} else {
		selectedSkills = available
	}

	result := &InstallResult{
		Skills: selectedSkills,
		Tools:  selectedTools,
		DryRun: input.DryRun,
	}

	// Dry run - return without installing
	if input.DryRun {
		return result, nil
	}

	// Install to each tool
	var lastInstalled []SkillInfo
	var installErrors []string
	successCount := 0

	for _, t := range selectedTools {
		// Get the appropriate skills directory based on scope
		skillsDir := GetScopedSkillsDir(t, scope, repoRoot)

		// For project scope, ensure the directory exists
		if scope == ScopeProject {
			if err := os.MkdirAll(skillsDir, 0o755); err != nil {
				installErrors = append(installErrors, fmt.Sprintf("%s: failed to create directory: %s", t.Name, err))
				continue
			}
		}

		installed, err := InstallSelectedSkills(skillsDir, tmpDir, selectedSkillNames)
		if err != nil {
			installErrors = append(installErrors, fmt.Sprintf("%s: %s", t.Name, err))
			continue
		}
		lastInstalled = installed
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("failed to install skills to any tool: %s", strings.Join(installErrors, "; "))
	}

	// Update result with actually installed skills
	result.Skills = lastInstalled

	// Save state
	SaveInstallStateWithScope(lastInstalled, selectedTools, tmpDir, scope)

	return result, nil
}

// SaveInstallState persists the skills state after a successful install (defaults to user scope).
func SaveInstallState(installed []SkillInfo, selectedTools []Tool, tmpDir string) {
	SaveInstallStateWithScope(installed, selectedTools, tmpDir, ScopeUser)
}

// SaveInstallStateWithScope persists the skills state after a successful install with a specific scope.
func SaveInstallStateWithScope(installed []SkillInfo, selectedTools []Tool, tmpDir string, scope Scope) {
	var newSkills []InstalledSkill
	for _, s := range installed {
		hash, err := HashSkillDir(filepath.Join(tmpDir, "skills", s.DirName))
		if err != nil {
			continue
		}
		newSkills = append(newSkills, s.ToInstalledWithScope(hash, scope))
	}

	// Load existing state to preserve skills from the other scope.
	// If loading fails (e.g. corrupted file), start fresh to avoid a nil dereference.
	existing, err := LoadState()
	if err != nil || existing == nil {
		existing = &SkillsState{}
	}

	// Build a set of newly installed dir names so we can replace them.
	newSet := make(map[string]bool, len(newSkills))
	for _, s := range newSkills {
		newSet[s.EffectiveDirName()] = true
	}

	// Keep existing skills that are from a different scope or not being replaced.
	var merged []InstalledSkill
	for _, sk := range existing.Skills {
		if sk.EffectiveScope() != scope || !newSet[sk.EffectiveDirName()] {
			merged = append(merged, sk)
		}
	}
	merged = append(merged, newSkills...)

	// Merge tool names (union of existing and newly selected).
	toolSet := make(map[string]bool)
	for _, name := range existing.Tools {
		toolSet[name] = true
	}
	for _, name := range ToolNames(selectedTools) {
		toolSet[name] = true
	}
	var mergedTools []string
	for name := range toolSet {
		mergedTools = append(mergedTools, name)
	}

	state := &SkillsState{
		Skills: merged,
		Tools:  mergedTools,
	}
	state.Touch()
	_ = state.Save()
}

// PrepareInstall handles the setup phase (detect tools, clone repo, read skills)
// without actually installing. Used by the TUI to get available options.
type PrepareResult struct {
	Tools     []Tool
	Skills    []SkillInfo
	TmpDir    string
	CleanupFn func()
}

// PrepareInstall detects tools and clones the repo to get available skills.
// Caller must call CleanupFn when done.
func PrepareInstall(toolFilter string) (*PrepareResult, error) {
	// Detect tools
	allTools, err := DetectTools()
	if err != nil {
		return nil, fmt.Errorf("failed to detect tools: %w", err)
	}
	if len(allTools) == 0 {
		return nil, fmt.Errorf("no supported AI coding tools detected")
	}

	var selectedTools []Tool
	if toolFilter != "" {
		selectedTools = FilterTools(allTools, toolFilter)
		if len(selectedTools) == 0 {
			return nil, fmt.Errorf("no installed tool matching %q found", toolFilter)
		}
	} else {
		selectedTools = allTools
	}

	// Clone repo
	tmpDir, err := os.MkdirTemp("", "render-skills-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := CloneSkillsRepo(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	available := ReadSkillsFromRepo(tmpDir)
	if len(available) == 0 {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("no skills found in the repository")
	}

	return &PrepareResult{
		Tools:     selectedTools,
		Skills:    available,
		TmpDir:    tmpDir,
		CleanupFn: func() { _ = os.RemoveAll(tmpDir) },
	}, nil
}

// CleanupTmpDir removes a temporary directory created during install.
func CleanupTmpDir(tmpDir string) {
	if tmpDir != "" {
		_ = os.RemoveAll(tmpDir)
	}
}

// ExecuteInstall performs the actual installation given prepared data.
// Used by the TUI after user has made selections.
func ExecuteInstall(tools []Tool, skillNames []string, tmpDir string) (*InstallResult, error) {
	return ExecuteInstallWithScope(tools, skillNames, tmpDir, ScopeUser, "")
}

// ExecuteInstallWithScope performs the actual installation with a specific scope.
// Used by the TUI after user has made selections.
func ExecuteInstallWithScope(tools []Tool, skillNames []string, tmpDir string, scope Scope, repoRoot string) (*InstallResult, error) {
	var lastInstalled []SkillInfo
	var installErrors []string
	successCount := 0

	for _, t := range tools {
		// Get the appropriate skills directory based on scope
		skillsDir := GetScopedSkillsDir(t, scope, repoRoot)

		// For project scope, ensure the directory exists
		if scope == ScopeProject {
			if err := os.MkdirAll(skillsDir, 0o755); err != nil {
				installErrors = append(installErrors, fmt.Sprintf("%s: failed to create directory: %s", t.Name, err))
				continue
			}
		}

		installed, err := InstallSelectedSkills(skillsDir, tmpDir, skillNames)
		if err != nil {
			installErrors = append(installErrors, fmt.Sprintf("%s: %s", t.Name, err))
			continue
		}
		lastInstalled = installed
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("failed to install skills to any tool: %s", strings.Join(installErrors, "; "))
	}

	// Save state
	SaveInstallStateWithScope(lastInstalled, tools, tmpDir, scope)

	return &InstallResult{
		Skills: lastInstalled,
		Tools:  tools,
		DryRun: false,
	}, nil
}

// ── State Loading ────────────────────────────────────────────────────────────

// LoadedState holds the result of loading skills state with fallback detection.
type LoadedState struct {
	State         *SkillsState
	DetectedTools []Tool
	Warnings      []string
}

// LoadOrRebuildState loads state from file, or rebuilds it from disk if missing.
// This consolidates the common pattern used across list/remove/update views.
func LoadOrRebuildState() (*LoadedState, error) {
	state, err := LoadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load skills state: %w", err)
	}

	detectedTools, err := DetectTools()
	if err != nil {
		return nil, fmt.Errorf("failed to detect tools: %w", err)
	}

	var warnings []string

	// If no state file, build from what's on disk
	if !state.HasSelections() && len(detectedTools) > 0 {
		allInstalled, toolNames, hashWarnings := ScanInstalledState(detectedTools)
		warnings = append(warnings, hashWarnings...)

		if len(allInstalled) > 0 {
			state.Tools = toolNames
			state.Skills = allInstalled
		}
	}

	return &LoadedState{
		State:         state,
		DetectedTools: detectedTools,
		Warnings:      warnings,
	}, nil
}

// ── Skill Name Resolution ────────────────────────────────────────────────────

// ResolveSkillNames converts skill names (or dir names) to canonical directory names.
// Returns both resolved names and unmatched inputs for error reporting.
func ResolveSkillNames(available []SkillInfo, filter []string) (resolved []string, unmatched []string) {
	nameToDir := make(map[string]string, len(available)*2)
	for _, s := range available {
		nameToDir[s.Name] = s.DirName
		nameToDir[s.DirName] = s.DirName
	}

	for _, name := range filter {
		if dirName, ok := nameToDir[name]; ok {
			resolved = append(resolved, dirName)
		} else {
			unmatched = append(unmatched, name)
		}
	}
	return resolved, unmatched
}

// ResolveInstalledSkillNames converts skill names to directory names based on installed skills.
// Returns resolved names and unmatched inputs for error reporting.
func ResolveInstalledSkillNames(installedSkills []InstalledSkill, names []string) (resolved []string, unmatched []string) {
	nameToDir := make(map[string]string, len(installedSkills)*2)
	for _, sk := range installedSkills {
		nameToDir[sk.Name] = sk.EffectiveDirName()
		nameToDir[sk.EffectiveDirName()] = sk.EffectiveDirName()
	}

	for _, name := range names {
		if dirName, ok := nameToDir[name]; ok {
			resolved = append(resolved, dirName)
		} else {
			unmatched = append(unmatched, name)
		}
	}
	return resolved, unmatched
}

// ── Tool Helpers ─────────────────────────────────────────────────────────────

// IntersectToolsByState returns only the detected tools that match the state's tool names.
func IntersectToolsByState(detectedTools []Tool, state *SkillsState) []Tool {
	detectedMap := make(map[string]Tool, len(detectedTools))
	for _, t := range detectedTools {
		detectedMap[t.Name] = t
	}

	var selected []Tool
	for _, name := range state.Tools {
		if t, ok := detectedMap[name]; ok {
			selected = append(selected, t)
		}
	}
	return selected
}

// FilterToolsByNames returns only the tools whose names are in the selectedNames list.
func FilterToolsByNames(allTools []Tool, selectedNames []string) []Tool {
	nameSet := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		nameSet[n] = true
	}

	var selected []Tool
	for _, t := range allTools {
		if nameSet[t.Name] {
			selected = append(selected, t)
		}
	}
	return selected
}

// FilterOutdatedByNames returns only the outdated skills whose DirName is in the selectedNames list.
func FilterOutdatedByNames(outdated []OutdatedSkill, selectedNames []string) []OutdatedSkill {
	nameSet := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		nameSet[n] = true
	}

	var selected []OutdatedSkill
	for _, o := range outdated {
		if nameSet[o.DirName] {
			selected = append(selected, o)
		}
	}
	return selected
}

// ── Skills Map Helpers ───────────────────────────────────────────────────────

// BuildSkillsMap creates a lookup map keyed by both Name and DirName.
func BuildSkillsMap(skills []SkillInfo) map[string]SkillInfo {
	m := make(map[string]SkillInfo, len(skills)*2)
	for _, s := range skills {
		m[s.Name] = s
		m[s.DirName] = s
	}
	return m
}

// ── Remove Operations ────────────────────────────────────────────────────────

// RemoveResult holds the outcome of a skills remove operation.
type RemoveResult struct {
	RemovedSkills []string
	Tools         []Tool
	Errors        []string
}

// ExecuteRemove removes specified skills from tools and updates state.
func ExecuteRemove(tools []Tool, skillDirNames []string, state *SkillsState) (*RemoveResult, error) {
	return ExecuteRemoveWithScope(tools, skillDirNames, state, ScopeUser, "")
}

// ExecuteRemoveWithScope removes specified skills from tools at a specific scope and updates state.
func ExecuteRemoveWithScope(tools []Tool, skillDirNames []string, state *SkillsState, scope Scope, repoRoot string) (*RemoveResult, error) {
	var removeErrors []string
	successCount := 0

	for _, t := range tools {
		// Get the appropriate skills directory based on scope
		skillsDir := GetScopedSkillsDir(t, scope, repoRoot)

		if err := RemoveSkills(skillsDir, skillDirNames); err != nil {
			removeErrors = append(removeErrors, fmt.Sprintf("%s: %s", t.Name, err))
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("failed to remove skills from any tool: %s", strings.Join(removeErrors, "; "))
	}

	// Update state
	UpdateStateAfterRemoval(state, skillDirNames, scope)

	return &RemoveResult{
		RemovedSkills: skillDirNames,
		Tools:         tools,
		Errors:        removeErrors,
	}, nil
}

// UpdateStateAfterRemoval removes specified skills from state and saves.
func UpdateStateAfterRemoval(state *SkillsState, removedDirNames []string, scope Scope) {
	removeSet := make(map[string]bool, len(removedDirNames))
	for _, name := range removedDirNames {
		removeSet[name] = true
	}

	var remaining []InstalledSkill
	for _, sk := range state.Skills {
		// Only remove skills matching both the dir name AND the scope.
		// Skills at other scopes are preserved.
		if removeSet[sk.EffectiveDirName()] && sk.EffectiveScope() == scope {
			continue
		}
		remaining = append(remaining, sk)
	}

	state.Skills = remaining
	state.Touch()
	_ = state.Save()
}

// ── Update Operations ────────────────────────────────────────────────────────

// OutdatedSkill represents a skill that has updates available.
type OutdatedSkill struct {
	Name       string // display name
	DirName    string // new directory name
	OldDirName string // previous directory name (may differ if renamed)
	Label      string // reason for update (e.g., "1.0 → 1.1" or "(content changed)")
}

// UpdateCheckResult holds the result of checking for skill updates.
type UpdateCheckResult struct {
	Outdated []OutdatedSkill
	UpToDate []string
	Warnings []string
}

// CheckForUpdates compares installed skills against remote versions.
func CheckForUpdates(state *SkillsState, remoteSkills []SkillInfo, tmpDir string, force bool) (*UpdateCheckResult, error) {
	remoteMap := BuildSkillsMap(remoteSkills)

	localHashMap := make(map[string]string, len(state.Skills))
	for _, sk := range state.Skills {
		localHashMap[sk.EffectiveDirName()] = sk.Hash
	}

	var outdated []OutdatedSkill
	var upToDate []string
	var warnings []string

	for _, sk := range state.Skills {
		// Look up by Name first, then by DirName for backward compat
		remote, exists := remoteMap[sk.Name]
		if !exists {
			remote, exists = remoteMap[sk.EffectiveDirName()]
		}
		if !exists {
			warnings = append(warnings, fmt.Sprintf("%s (no longer in repository, skipping)", sk.Name))
			continue
		}

		remoteHash, err := HashSkillDir(filepath.Join(tmpDir, "skills", remote.DirName))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s (failed to hash remote: %s)", sk.Name, err))
			continue
		}

		localHash := localHashMap[sk.EffectiveDirName()]

		if localHash == remoteHash && !force {
			upToDate = append(upToDate, sk.Name)
		} else {
			var reason string
			if force && localHash == remoteHash {
				reason = "(forced reinstall)"
			} else if sk.Version != remote.Version() {
				reason = fmt.Sprintf("%s → %s", sk.Version, remote.Version())
			} else {
				reason = "(content changed)"
			}
			outdated = append(outdated, OutdatedSkill{
				Name:       sk.Name,
				DirName:    remote.DirName,
				OldDirName: sk.EffectiveDirName(),
				Label:      reason,
			})
		}
	}

	return &UpdateCheckResult{
		Outdated: outdated,
		UpToDate: upToDate,
		Warnings: warnings,
	}, nil
}

// UpdateResult holds the outcome of a skills update operation.
type UpdateResult struct {
	UpdatedSkills []OutdatedSkill
	Tools         []Tool
	Errors        []string
}

// ExecuteUpdate performs the actual update of outdated skills.
// It handles renames by removing old directories before installing new ones.
func ExecuteUpdate(tools []Tool, outdated []OutdatedSkill, tmpDir string) (*UpdateResult, error) {
	return ExecuteUpdateWithScope(tools, outdated, tmpDir, ScopeUser, "")
}

// ExecuteUpdateWithScope performs the actual update of outdated skills at a specific scope.
// It handles renames by removing old directories before installing new ones.
func ExecuteUpdateWithScope(tools []Tool, outdated []OutdatedSkill, tmpDir string, scope Scope, repoRoot string) (*UpdateResult, error) {
	// Build list of skill dir names to install
	skillDirNames := make([]string, len(outdated))
	for i, o := range outdated {
		skillDirNames[i] = o.DirName
	}

	// Collect old directories that need removal for renamed skills
	var renamedOldDirs []string
	for _, o := range outdated {
		if o.OldDirName != o.DirName {
			renamedOldDirs = append(renamedOldDirs, o.OldDirName)
		}
	}

	var updateErrors []string
	successCount := 0

	for _, t := range tools {
		// Get the appropriate skills directory based on scope
		skillsDir := GetScopedSkillsDir(t, scope, repoRoot)

		// For project scope, ensure the directory exists
		if scope == ScopeProject {
			if err := os.MkdirAll(skillsDir, 0o755); err != nil {
				updateErrors = append(updateErrors, fmt.Sprintf("%s: failed to create directory: %s", t.Name, err))
				continue
			}
		}

		// Remove old directories for renamed skills before installing,
		// since InstallSelectedSkills only removes by the new names.
		if len(renamedOldDirs) > 0 {
			_ = RemoveSkills(skillsDir, renamedOldDirs)
		}

		_, err := InstallSelectedSkills(skillsDir, tmpDir, skillDirNames)
		if err != nil {
			updateErrors = append(updateErrors, fmt.Sprintf("%s: %s", t.Name, err))
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("failed to update skills on any tool: %s", strings.Join(updateErrors, "; "))
	}

	return &UpdateResult{
		UpdatedSkills: outdated,
		Tools:         tools,
		Errors:        updateErrors,
	}, nil
}

// UpdateStateAfterUpdate updates version and hash information for updated skills.
// Only state entries matching the given scope are modified so that skills at
// other scopes are not incorrectly marked as up to date.
func UpdateStateAfterUpdate(state *SkillsState, updatedSkills []OutdatedSkill, remoteSkills []SkillInfo, tmpDir string, scope Scope) {
	remoteMap := BuildSkillsMap(remoteSkills)

	// Create mapping from old to new dir names
	oldToNew := make(map[string]string)
	for _, u := range updatedSkills {
		oldToNew[u.OldDirName] = u.DirName
	}

	// Update state entries — only for the scope that was actually updated.
	for i, sk := range state.Skills {
		if sk.EffectiveScope() != scope {
			continue
		}
		dirName := sk.EffectiveDirName()
		newDirName, matched := oldToNew[dirName]
		if !matched {
			continue
		}

		// Look up by the new directory name (remoteMap is keyed by remote
		// DirName, which may differ from the old local dirName after a rename).
		remote, ok := remoteMap[newDirName]
		if !ok {
			remote, ok = remoteMap[sk.Name]
		}
		if ok {
			state.Skills[i].Version = remote.Version()
			state.Skills[i].DirName = remote.DirName

			hash, err := HashSkillDir(filepath.Join(tmpDir, "skills", remote.DirName))
			if err == nil {
				state.Skills[i].Hash = hash
			}
		}
	}

	state.Touch()
	_ = state.Save()
}
