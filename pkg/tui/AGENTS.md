# TUI Development Guide

> **Last verified**: 2026-02-10

Detailed patterns for Bubble Tea TUI development in the Render CLI.

For boundaries (Never/Ask/Safe) and escalation guidance, see the [main AGENTS.md](../../AGENTS.md#boundaries).

## Architecture

The CLI uses the [Elm Architecture](https://guide.elm-lang.org/architecture/):
`Message → Update(model) → View(model) → Render`

Every component implements: `Init()`, `Update(msg tea.Msg)`, `View()`

---

## Stack-based Navigation

The `StackModel` in `stack.go` manages view navigation with breadcrumbs.

```go
// Push returns a tea.Cmd - must be returned from Update()
cmd := stack.Push(ModelWithCmd{
    Model:      myModel,
    Cmd:        "render services list",  // For clipboard
    Breadcrumb: "Services",
})
return m, cmd
```

---

## Message Patterns

```go
// Name messages with Action + Msg suffix
type LoadingDataMsg struct{}
type DataLoadedMsg struct{ Data []Item }
type ErrorMsg struct{ Err error }
```

---

## Async Commands

Never block in `Update()`. Use commands for I/O:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m, m.fetchData()  // Return command, don't block
    case DataLoadedMsg:
        m.data = msg.Data
        return m, nil
    }
    return m, nil
}

func (m Model) fetchData() tea.Cmd {
    return func() tea.Msg {
        data, err := m.repo.List()
        if err != nil {
            return ErrorMsg{Err: err}
        }
        return DataLoadedMsg{Data: data}
    }
}
```

---

## Subcomponent Updates

Always delegate updates to child components:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    var cmd tea.Cmd

    m.table, cmd = m.table.Update(msg)
    cmds = append(cmds, cmd)

    m.input, cmd = m.input.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

---

## Debugging & Testing

```go
// Log to file (stdout is occupied by TUI)
f, _ := tea.LogToFile("debug.log", "debug")
defer f.Close()
```

```bash
tail -f debug.log          # Watch logs in another terminal
go test ./pkg/tui/...      # Run TUI tests
```

If terminal breaks after crash, run `reset`.

**Test patterns**: Table-driven tests with `stretchr/testify`, manual fakes in `testhelper/`

```go
func TestMyView(t *testing.T) {
    fake := &testhelper.FakeDimensionModel{Value: "test"}
    model := NewMyModel(fake)
    // Assert on View() output or model state
}
```

---

## Styling

Use `pkg/style/` for consistent styling. Never hardcode dimensions—use `lipgloss.Height()` and `lipgloss.Width()`.

```go
title := style.Title.Render("My Title")
height := lipgloss.Height(rendered)
```

---

## Common Mistakes

- Blocking in `Update()` - use commands for async work
- Forgetting to handle `tea.KeyCtrlC` and `tea.KeyCtrlD`
- Not returning the `tea.Cmd` from `Push()`
- Hardcoding dimensions instead of using lipgloss
- Message ordering from concurrent commands is undefined

---

## Reference

[Bubble Tea](https://github.com/charmbracelet/bubbletea) | [Lipgloss](https://github.com/charmbracelet/lipgloss) | [Bubbles](https://github.com/charmbracelet/bubbles)
