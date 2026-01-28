package wizard

import (
	"testing"
)

func TestSelectStep(t *testing.T) {
	options := []SelectOption{
		{Value: "a", Label: "Option A", Description: "First option"},
		{Value: "b", Label: "Option B", Description: "Second option"},
	}

	step := NewSelectStep("test", "Test Step", options).
		WithDescription("Choose an option")

	if step.ID() != "test" {
		t.Errorf("expected ID 'test', got %s", step.ID())
	}

	if step.Title() != "Test Step" {
		t.Errorf("expected Title 'Test Step', got %s", step.Title())
	}

	if step.Skip(nil) {
		t.Error("expected Skip to return false by default")
	}

	// Test skip function
	stepWithSkip := NewSelectStep("skip", "Skip Step", options).
		WithSkipFunc(func(s State) bool { return true })

	if !stepWithSkip.Skip(nil) {
		t.Error("expected Skip to return true when skipFunc returns true")
	}
}

func TestConfirmStep(t *testing.T) {
	step := NewConfirmStep("confirm", "Confirm?").
		WithDefault(false)

	if step.ID() != "confirm" {
		t.Errorf("expected ID 'confirm', got %s", step.ID())
	}

	model := step.Init(nil)
	if m, ok := model.(*confirmModel); ok {
		if m.defaultVal != false {
			t.Error("expected default value to be false")
		}
	} else {
		t.Error("expected confirmModel type")
	}
}

func TestInputStep(t *testing.T) {
	step := NewInputStep("input", "Enter value").
		WithDefault("default").
		WithPlaceholder("Type here...")

	if step.ID() != "input" {
		t.Errorf("expected ID 'input', got %s", step.ID())
	}

	model := step.Init(nil)
	if m, ok := model.(*inputModel); ok {
		if m.textInput.Value() != "default" {
			t.Errorf("expected default value 'default', got %s", m.textInput.Value())
		}
	} else {
		t.Error("expected inputModel type")
	}
}

func TestMultiSelectStep(t *testing.T) {
	options := []SelectOption{
		{Value: "a", Label: "Option A"},
		{Value: "b", Label: "Option B"},
		{Value: "c", Label: "Option C"},
	}

	step := NewMultiSelectStep("multi", "Select multiple", options).
		WithDefaults([]string{"a", "c"})

	model := step.Init(nil)
	if m, ok := model.(*multiSelectModel); ok {
		if !m.selected[0] {
			t.Error("expected option A to be pre-selected")
		}
		if m.selected[1] {
			t.Error("expected option B to NOT be pre-selected")
		}
		if !m.selected[2] {
			t.Error("expected option C to be pre-selected")
		}
	} else {
		t.Error("expected multiSelectModel type")
	}
}

func TestDisplayStep(t *testing.T) {
	step := NewDisplayStep("display", "Info", func(s State) string {
		return "This is some information"
	})

	if step.ID() != "display" {
		t.Errorf("expected ID 'display', got %s", step.ID())
	}

	model := step.Init(nil)
	if m, ok := model.(*displayModel); ok {
		if m.content != "This is some information" {
			t.Errorf("expected content 'This is some information', got %s", m.content)
		}
	} else {
		t.Error("expected displayModel type")
	}
}

func TestWizardState(t *testing.T) {
	state := make(State)
	state["key1"] = "value1"
	state["key2"] = 42

	if state["key1"] != "value1" {
		t.Error("expected key1 to be value1")
	}

	if state["key2"] != 42 {
		t.Error("expected key2 to be 42")
	}
}

func TestWizardNew(t *testing.T) {
	step1 := NewSelectStep("step1", "Step 1", []SelectOption{
		{Value: "a", Label: "A"},
	})
	step2 := NewConfirmStep("step2", "Confirm?")

	w := New(step1, step2)

	if len(w.steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(w.steps))
	}

	state := w.State()
	if state == nil {
		t.Error("expected state to be initialized")
	}
}

func TestWizardWithState(t *testing.T) {
	w := New()
	initialState := State{"preset": "value"}
	w.WithState(initialState)

	if w.state["preset"] != "value" {
		t.Error("expected preset state to be set")
	}
}
