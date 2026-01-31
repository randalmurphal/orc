// Package wizard provides a Bubbletea-based wizard framework for interactive CLI setup.
package wizard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// State holds the wizard's collected data.
// Each step reads from and writes to this shared state.
type State map[string]any

// Step represents a single wizard step.
type Step interface {
	// ID returns a unique identifier for this step.
	ID() string

	// Title returns the step's title shown in the header.
	Title() string

	// Description returns optional description text.
	Description() string

	// Skip returns true if this step should be skipped based on current state.
	Skip(state State) bool

	// Init creates the initial model for this step.
	Init(state State) tea.Model

	// Result extracts the result from the model and stores it in state.
	// Called when the step completes successfully.
	Result(model tea.Model, state State)
}

// Wizard manages a sequence of steps.
type Wizard struct {
	steps   []Step
	current int
	state   State
	model   tea.Model
	done    bool
	err     error

	// Styling
	styles Styles
}

// Styles contains the visual styling for the wizard.
type Styles struct {
	Title       lipgloss.Style
	Description lipgloss.Style
	Progress    lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Subtle      lipgloss.Style
}

// DefaultStyles returns the default wizard styling.
func DefaultStyles() Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1),
		Description: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginBottom(1),
		Progress: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
	}
}

// New creates a new wizard with the given steps.
func New(steps ...Step) *Wizard {
	return &Wizard{
		steps:  steps,
		state:  make(State),
		styles: DefaultStyles(),
	}
}

// WithState sets the initial state for the wizard.
func (w *Wizard) WithState(state State) *Wizard {
	w.state = state
	return w
}

// WithStyles sets custom styling for the wizard.
func (w *Wizard) WithStyles(styles Styles) *Wizard {
	w.styles = styles
	return w
}

// State returns the wizard's current state.
func (w *Wizard) State() State {
	return w.state
}

// Run executes the wizard interactively.
func (w *Wizard) Run() error {
	// Skip steps that should be skipped
	w.skipToNextStep()

	if w.current >= len(w.steps) {
		return nil // No steps to run
	}

	// Initialize the first step's model
	w.model = w.steps[w.current].Init(w.state)

	// Run the bubbletea program
	p := tea.NewProgram(w)
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("wizard error: %w", err)
	}

	return w.err
}

// Init implements tea.Model.
func (w *Wizard) Init() tea.Cmd {
	if w.model == nil {
		return nil
	}
	return w.model.Init()
}

// Update implements tea.Model.
func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			w.err = fmt.Errorf("wizard cancelled")
			return w, tea.Quit
		}

	case StepCompleteMsg:
		// Save the result
		w.steps[w.current].Result(w.model, w.state)

		// Move to next step
		w.current++
		w.skipToNextStep()

		if w.current >= len(w.steps) {
			w.done = true
			return w, tea.Quit
		}

		// Initialize the next step
		w.model = w.steps[w.current].Init(w.state)
		return w, w.model.Init()

	case StepCancelMsg:
		w.err = fmt.Errorf("step cancelled: %s", msg.Reason)
		return w, tea.Quit
	}

	// Pass to current step's model
	if w.model != nil {
		var cmd tea.Cmd
		w.model, cmd = w.model.Update(msg)
		return w, cmd
	}

	return w, nil
}

// View implements tea.Model.
func (w *Wizard) View() string {
	if w.current >= len(w.steps) {
		return ""
	}

	step := w.steps[w.current]
	var s string

	// Progress indicator
	progress := fmt.Sprintf("Step %d of %d", w.current+1, len(w.steps))
	s += w.styles.Progress.Render(progress) + "\n\n"

	// Title
	s += w.styles.Title.Render(step.Title()) + "\n"

	// Description
	if desc := step.Description(); desc != "" {
		s += w.styles.Description.Render(desc) + "\n"
	}

	// Step content
	if w.model != nil {
		s += w.model.View()
	}

	return s
}

// skipToNextStep advances to the next non-skipped step.
func (w *Wizard) skipToNextStep() {
	for w.current < len(w.steps) && w.steps[w.current].Skip(w.state) {
		w.current++
	}
}

// StepCompleteMsg signals that the current step is complete.
type StepCompleteMsg struct{}

// StepCancelMsg signals that the current step was cancelled.
type StepCancelMsg struct {
	Reason string
}

// CompleteStep returns a command that signals step completion.
func CompleteStep() tea.Cmd {
	return func() tea.Msg {
		return StepCompleteMsg{}
	}
}

