package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------- Select Step ----------------------

// SelectOption represents a single selectable option.
type SelectOption struct {
	Value       string
	Label       string
	Description string
}

// SelectStep allows the user to choose one option from a list.
type SelectStep struct {
	id          string
	title       string
	description string
	options     []SelectOption
	stateKey    string
	skipFunc    func(State) bool
}

// NewSelectStep creates a new select step.
func NewSelectStep(id, title string, options []SelectOption) *SelectStep {
	return &SelectStep{
		id:       id,
		title:    title,
		options:  options,
		stateKey: id,
	}
}

// WithDescription sets the step description.
func (s *SelectStep) WithDescription(desc string) *SelectStep {
	s.description = desc
	return s
}

// WithStateKey sets the key where the result is stored in state.
func (s *SelectStep) WithStateKey(key string) *SelectStep {
	s.stateKey = key
	return s
}

// WithSkipFunc sets a function to determine if this step should be skipped.
func (s *SelectStep) WithSkipFunc(fn func(State) bool) *SelectStep {
	s.skipFunc = fn
	return s
}

func (s *SelectStep) ID() string          { return s.id }
func (s *SelectStep) Title() string       { return s.title }
func (s *SelectStep) Description() string { return s.description }

func (s *SelectStep) Skip(state State) bool {
	if s.skipFunc != nil {
		return s.skipFunc(state)
	}
	return false
}

func (s *SelectStep) Init(state State) tea.Model {
	return &selectModel{
		options:  s.options,
		cursor:   0,
		selected: -1,
	}
}

func (s *SelectStep) Result(model tea.Model, state State) {
	if m, ok := model.(*selectModel); ok && m.selected >= 0 {
		state[s.stateKey] = m.options[m.selected].Value
	}
}

type selectModel struct {
	options  []SelectOption
	cursor   int
	selected int
}

func (m *selectModel) Init() tea.Cmd { return nil }

func (m *selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.selected = m.cursor
			return m, CompleteStep()
		}
	}
	return m, nil
}

func (m *selectModel) View() string {
	var b strings.Builder

	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		line := cursor + opt.Label
		if opt.Description != "" {
			line += " - " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(opt.Description)
		}

		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓: navigate • enter: select"))

	return b.String()
}

// ---------------------- Confirm Step ----------------------

// ConfirmStep asks the user a yes/no question.
type ConfirmStep struct {
	id          string
	title       string
	description string
	defaultVal  bool
	stateKey    string
	skipFunc    func(State) bool
}

// NewConfirmStep creates a new confirmation step.
func NewConfirmStep(id, title string) *ConfirmStep {
	return &ConfirmStep{
		id:         id,
		title:      title,
		defaultVal: true,
		stateKey:   id,
	}
}

// WithDescription sets the step description.
func (s *ConfirmStep) WithDescription(desc string) *ConfirmStep {
	s.description = desc
	return s
}

// WithDefault sets the default value.
func (s *ConfirmStep) WithDefault(val bool) *ConfirmStep {
	s.defaultVal = val
	return s
}

// WithStateKey sets the key where the result is stored in state.
func (s *ConfirmStep) WithStateKey(key string) *ConfirmStep {
	s.stateKey = key
	return s
}

// WithSkipFunc sets a function to determine if this step should be skipped.
func (s *ConfirmStep) WithSkipFunc(fn func(State) bool) *ConfirmStep {
	s.skipFunc = fn
	return s
}

func (s *ConfirmStep) ID() string          { return s.id }
func (s *ConfirmStep) Title() string       { return s.title }
func (s *ConfirmStep) Description() string { return s.description }

func (s *ConfirmStep) Skip(state State) bool {
	if s.skipFunc != nil {
		return s.skipFunc(state)
	}
	return false
}

func (s *ConfirmStep) Init(state State) tea.Model {
	return &confirmModel{
		value:      s.defaultVal,
		defaultVal: s.defaultVal,
	}
}

func (s *ConfirmStep) Result(model tea.Model, state State) {
	if m, ok := model.(*confirmModel); ok {
		state[s.stateKey] = m.value
	}
}

type confirmModel struct {
	value      bool
	defaultVal bool
}

func (m *confirmModel) Init() tea.Cmd { return nil }

func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.value = true
			return m, CompleteStep()
		case "n", "N":
			m.value = false
			return m, CompleteStep()
		case "enter":
			return m, CompleteStep()
		case "left", "h":
			m.value = true
		case "right", "l":
			m.value = false
		}
	}
	return m, nil
}

func (m *confirmModel) View() string {
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	var yes, no string
	if m.value {
		yes = selectedStyle.Render("[Yes]")
		no = normalStyle.Render(" No ")
	} else {
		yes = normalStyle.Render(" Yes ")
		no = selectedStyle.Render("[No]")
	}

	return fmt.Sprintf("%s / %s\n\n%s",
		yes, no,
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("y/n: select • ←/→: toggle • enter: confirm"))
}

// ---------------------- Input Step ----------------------

// InputStep allows the user to enter text.
type InputStep struct {
	id           string
	title        string
	description  string
	placeholder  string
	defaultValue string
	stateKey     string
	skipFunc     func(State) bool
	validate     func(string) error
}

// NewInputStep creates a new text input step.
func NewInputStep(id, title string) *InputStep {
	return &InputStep{
		id:       id,
		title:    title,
		stateKey: id,
	}
}

// WithPlaceholder sets the placeholder text.
func (s *InputStep) WithPlaceholder(placeholder string) *InputStep {
	s.placeholder = placeholder
	return s
}

// WithDefault sets the default value.
func (s *InputStep) WithDefault(val string) *InputStep {
	s.defaultValue = val
	return s
}

func (s *InputStep) Init(state State) tea.Model {
	ti := textinput.New()
	ti.Placeholder = s.placeholder
	ti.SetValue(s.defaultValue)
	ti.Focus()
	ti.Width = 50

	return &inputModel{
		textInput: ti,
		validate:  s.validate,
	}
}

type inputModel struct {
	textInput textinput.Model
	validate  func(string) error
	err       error
}

func (m *inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.validate != nil {
				if err := m.validate(m.textInput.Value()); err != nil {
					m.err = err
					return m, nil
				}
			}
			return m, CompleteStep()
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *inputModel) View() string {
	var s string
	s += m.textInput.View() + "\n\n"

	if m.err != nil {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: "+m.err.Error()) + "\n"
	}

	s += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("enter: confirm")

	return s
}

// ---------------------- Multi-Select Step ----------------------

// MultiSelectStep allows the user to choose multiple options from a list.
type MultiSelectStep struct {
	id          string
	title       string
	description string
	options     []SelectOption
	stateKey    string
	skipFunc    func(State) bool
	defaults    []string // Values of options that should be pre-selected
}

// NewMultiSelectStep creates a new multi-select step.
func NewMultiSelectStep(id, title string, options []SelectOption) *MultiSelectStep {
	return &MultiSelectStep{
		id:       id,
		title:    title,
		options:  options,
		stateKey: id,
	}
}

// WithDescription sets the step description.
func (s *MultiSelectStep) WithDescription(desc string) *MultiSelectStep {
	s.description = desc
	return s
}

// WithStateKey sets the key where the result is stored in state.
func (s *MultiSelectStep) WithStateKey(key string) *MultiSelectStep {
	s.stateKey = key
	return s
}

// WithSkipFunc sets a function to determine if this step should be skipped.
func (s *MultiSelectStep) WithSkipFunc(fn func(State) bool) *MultiSelectStep {
	s.skipFunc = fn
	return s
}

// WithDefaults sets the default selected values.
func (s *MultiSelectStep) WithDefaults(values []string) *MultiSelectStep {
	s.defaults = values
	return s
}

func (s *MultiSelectStep) ID() string          { return s.id }
func (s *MultiSelectStep) Title() string       { return s.title }
func (s *MultiSelectStep) Description() string { return s.description }

func (s *MultiSelectStep) Skip(state State) bool {
	if s.skipFunc != nil {
		return s.skipFunc(state)
	}
	return false
}

func (s *MultiSelectStep) Init(state State) tea.Model {
	selected := make(map[int]bool)

	// Pre-select defaults
	for _, def := range s.defaults {
		for i, opt := range s.options {
			if opt.Value == def {
				selected[i] = true
			}
		}
	}

	return &multiSelectModel{
		options:  s.options,
		selected: selected,
		cursor:   0,
	}
}

func (s *MultiSelectStep) Result(model tea.Model, state State) {
	if m, ok := model.(*multiSelectModel); ok {
		var values []string
		for i, opt := range m.options {
			if m.selected[i] {
				values = append(values, opt.Value)
			}
		}
		state[s.stateKey] = values
	}
}

type multiSelectModel struct {
	options  []SelectOption
	selected map[int]bool
	cursor   int
}

func (m *multiSelectModel) Init() tea.Cmd { return nil }

func (m *multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case " ", "x":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "enter":
			return m, CompleteStep()
		case "a":
			// Select all
			for i := range m.options {
				m.selected[i] = true
			}
		case "n":
			// Select none
			m.selected = make(map[int]bool)
		}
	}
	return m, nil
}

func (m *multiSelectModel) View() string {
	var b strings.Builder

	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		checkbox := "[ ]"
		if m.selected[i] {
			checkbox = "[x]"
		}

		line := cursor + checkbox + " " + opt.Label
		if opt.Description != "" {
			line += " - " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(opt.Description)
		}

		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓: navigate • space: toggle • a: all • n: none • enter: confirm"))

	return b.String()
}

// ---------------------- Display Step ----------------------

// DisplayStep shows information without requiring input.
type DisplayStep struct {
	id          string
	title       string
	description string
	content     func(State) string
	skipFunc    func(State) bool
}

// NewDisplayStep creates a new display step.
func NewDisplayStep(id, title string, content func(State) string) *DisplayStep {
	return &DisplayStep{
		id:      id,
		title:   title,
		content: content,
	}
}

// WithDescription sets the step description.
func (s *DisplayStep) WithDescription(desc string) *DisplayStep {
	s.description = desc
	return s
}

// WithSkipFunc sets a function to determine if this step should be skipped.
func (s *DisplayStep) WithSkipFunc(fn func(State) bool) *DisplayStep {
	s.skipFunc = fn
	return s
}

func (s *DisplayStep) ID() string          { return s.id }
func (s *DisplayStep) Title() string       { return s.title }
func (s *DisplayStep) Description() string { return s.description }

func (s *DisplayStep) Skip(state State) bool {
	if s.skipFunc != nil {
		return s.skipFunc(state)
	}
	return false
}

func (s *DisplayStep) Init(state State) tea.Model {
	return &displayModel{
		content: s.content(state),
	}
}

func (s *DisplayStep) Result(model tea.Model, state State) {
	// Display steps don't produce results
}

type displayModel struct {
	content string
}

func (m *displayModel) Init() tea.Cmd { return nil }

func (m *displayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			return m, CompleteStep()
		}
	}
	return m, nil
}

func (m *displayModel) View() string {
	return m.content + "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("enter: continue")
}
