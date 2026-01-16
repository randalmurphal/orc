package initiative

import (
	"strings"
	"testing"
)

func TestParseManifestBytes(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid manifest with initiative",
			yaml: `
version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "First task"
    weight: small
`,
			wantErr: false,
		},
		{
			name: "valid manifest with create_initiative",
			yaml: `
version: 1
create_initiative:
  title: "New Initiative"
  vision: "Make things better"
tasks:
  - id: 1
    title: "First task"
`,
			wantErr: false,
		},
		{
			name: "invalid yaml",
			yaml: `
version: 1
initiative: INIT-001
tasks:
  - id: not_a_number
    title: "First task"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifestBytes([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseManifestBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name      string
		manifest  *Manifest
		wantErrs  int
		errSubstr string // Substring that should appear in at least one error
	}{
		{
			name: "valid manifest",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
					{ID: 2, Title: "Task 2", DependsOn: []int{1}},
				},
			},
			wantErrs: 0,
		},
		{
			name: "missing version",
			manifest: &Manifest{
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
				},
			},
			wantErrs:  1,
			errSubstr: "version",
		},
		{
			name: "unsupported version",
			manifest: &Manifest{
				Version:    99,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
				},
			},
			wantErrs:  1,
			errSubstr: "unsupported version",
		},
		{
			name: "missing initiative",
			manifest: &Manifest{
				Version: 1,
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
				},
			},
			wantErrs:  1,
			errSubstr: "initiative",
		},
		{
			name: "both initiative and create_initiative",
			manifest: &Manifest{
				Version:          1,
				Initiative:       "INIT-001",
				CreateInitiative: &CreateInitiative{Title: "New"},
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
				},
			},
			wantErrs:  1,
			errSubstr: "mutually exclusive",
		},
		{
			name: "create_initiative missing title",
			manifest: &Manifest{
				Version:          1,
				CreateInitiative: &CreateInitiative{Vision: "Something"},
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
				},
			},
			wantErrs:  1,
			errSubstr: "create_initiative.title",
		},
		{
			name: "no tasks",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks:      []ManifestTask{},
			},
			wantErrs:  1,
			errSubstr: "at least one task",
		},
		{
			name: "task missing id",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{Title: "Task without ID"},
				},
			},
			wantErrs:  1,
			errSubstr: "local ID is required",
		},
		{
			name: "task missing title",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1},
				},
			},
			wantErrs:  1,
			errSubstr: "title is required",
		},
		{
			name: "duplicate local IDs",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1"},
					{ID: 1, Title: "Task 2"},
				},
			},
			wantErrs:  1,
			errSubstr: "duplicate local ID",
		},
		{
			name: "invalid weight",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1", Weight: "invalid"},
				},
			},
			wantErrs:  1,
			errSubstr: "invalid weight",
		},
		{
			name: "invalid category",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1", Category: "invalid"},
				},
			},
			wantErrs:  1,
			errSubstr: "invalid category",
		},
		{
			name: "invalid priority",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1", Priority: "invalid"},
				},
			},
			wantErrs:  1,
			errSubstr: "invalid priority",
		},
		{
			name: "dependency references unknown ID",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1", DependsOn: []int{99}},
				},
			},
			wantErrs:  1,
			errSubstr: "unknown local ID",
		},
		{
			name: "self dependency",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{ID: 1, Title: "Task 1", DependsOn: []int{1}},
				},
			},
			wantErrs:  2, // Both self-reference and circular dependency detected
			errSubstr: "cannot depend on itself",
		},
		{
			name: "valid all fields",
			manifest: &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks: []ManifestTask{
					{
						ID:          1,
						Title:       "Task 1",
						Description: "Do something",
						Weight:      "medium",
						Category:    "feature",
						Priority:    "high",
						Spec:        "# Spec\n\nDo this thing",
					},
					{
						ID:        2,
						Title:     "Task 2",
						DependsOn: []int{1},
					},
				},
			},
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateManifest(tt.manifest)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateManifest() got %d errors, want %d", len(errs), tt.wantErrs)
				for _, e := range errs {
					t.Logf("  error: %v", e)
				}
			}
			if tt.errSubstr != "" && len(errs) > 0 {
				found := false
				for _, e := range errs {
					if strings.Contains(e.Error(), tt.errSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, but none found", tt.errSubstr)
				}
			}
		})
	}
}

func TestDetectCircularDependencies(t *testing.T) {
	tests := []struct {
		name      string
		tasks     []ManifestTask
		wantCycle bool
	}{
		{
			name: "no dependencies",
			tasks: []ManifestTask{
				{ID: 1, Title: "A"},
				{ID: 2, Title: "B"},
			},
			wantCycle: false,
		},
		{
			name: "linear chain",
			tasks: []ManifestTask{
				{ID: 1, Title: "A"},
				{ID: 2, Title: "B", DependsOn: []int{1}},
				{ID: 3, Title: "C", DependsOn: []int{2}},
			},
			wantCycle: false,
		},
		{
			name: "diamond pattern",
			tasks: []ManifestTask{
				{ID: 1, Title: "A"},
				{ID: 2, Title: "B", DependsOn: []int{1}},
				{ID: 3, Title: "C", DependsOn: []int{1}},
				{ID: 4, Title: "D", DependsOn: []int{2, 3}},
			},
			wantCycle: false,
		},
		{
			name: "simple cycle",
			tasks: []ManifestTask{
				{ID: 1, Title: "A", DependsOn: []int{2}},
				{ID: 2, Title: "B", DependsOn: []int{1}},
			},
			wantCycle: true,
		},
		{
			name: "three node cycle",
			tasks: []ManifestTask{
				{ID: 1, Title: "A", DependsOn: []int{3}},
				{ID: 2, Title: "B", DependsOn: []int{1}},
				{ID: 3, Title: "C", DependsOn: []int{2}},
			},
			wantCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to validate to trigger cycle detection
			m := &Manifest{
				Version:    1,
				Initiative: "INIT-001",
				Tasks:      tt.tasks,
			}
			errs := ValidateManifest(m)

			hasCycleErr := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "circular dependency") {
					hasCycleErr = true
					break
				}
			}

			if hasCycleErr != tt.wantCycle {
				t.Errorf("cycle detection: got %v, want %v", hasCycleErr, tt.wantCycle)
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name    string
		tasks   []ManifestTask
		wantErr bool
		// validate checks that dependencies come before dependents
		validate func([]int, []ManifestTask) bool
	}{
		{
			name: "no dependencies",
			tasks: []ManifestTask{
				{ID: 1, Title: "A"},
				{ID: 2, Title: "B"},
				{ID: 3, Title: "C"},
			},
			wantErr: false,
			validate: func(order []int, tasks []ManifestTask) bool {
				return len(order) == len(tasks)
			},
		},
		{
			name: "linear chain",
			tasks: []ManifestTask{
				{ID: 1, Title: "A"},
				{ID: 2, Title: "B", DependsOn: []int{1}},
				{ID: 3, Title: "C", DependsOn: []int{2}},
			},
			wantErr: false,
			validate: func(order []int, tasks []ManifestTask) bool {
				// Task at index 0 should come first (ID=1)
				// Task at index 1 should come second (ID=2, depends on 1)
				// Task at index 2 should come last (ID=3, depends on 2)
				if len(order) != 3 {
					return false
				}
				// Verify order preserves dependencies
				pos := make(map[int]int)
				for i, idx := range order {
					pos[tasks[idx].ID] = i
				}
				// 1 should come before 2
				if pos[1] >= pos[2] {
					return false
				}
				// 2 should come before 3
				if pos[2] >= pos[3] {
					return false
				}
				return true
			},
		},
		{
			name: "diamond",
			tasks: []ManifestTask{
				{ID: 1, Title: "A"},
				{ID: 2, Title: "B", DependsOn: []int{1}},
				{ID: 3, Title: "C", DependsOn: []int{1}},
				{ID: 4, Title: "D", DependsOn: []int{2, 3}},
			},
			wantErr: false,
			validate: func(order []int, tasks []ManifestTask) bool {
				if len(order) != 4 {
					return false
				}
				pos := make(map[int]int)
				for i, idx := range order {
					pos[tasks[idx].ID] = i
				}
				// 1 should come before 2, 3, and 4
				if pos[1] >= pos[2] || pos[1] >= pos[3] || pos[1] >= pos[4] {
					return false
				}
				// 2 and 3 should come before 4
				if pos[2] >= pos[4] || pos[3] >= pos[4] {
					return false
				}
				return true
			},
		},
		{
			name: "cycle",
			tasks: []ManifestTask{
				{ID: 1, Title: "A", DependsOn: []int{2}},
				{ID: 2, Title: "B", DependsOn: []int{1}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := TopologicalSort(tt.tasks)
			if (err != nil) != tt.wantErr {
				t.Errorf("TopologicalSort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validate != nil {
				if !tt.validate(order, tt.tasks) {
					t.Errorf("TopologicalSort() order validation failed: %v", order)
				}
			}
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	// Just ensure they don't panic and return non-empty strings
	weights := formatWeights()
	if weights == "" {
		t.Error("formatWeights() returned empty string")
	}
	if !strings.Contains(weights, "medium") {
		t.Error("formatWeights() should contain 'medium'")
	}

	cats := formatCategories()
	if cats == "" {
		t.Error("formatCategories() returned empty string")
	}
	if !strings.Contains(cats, "feature") {
		t.Error("formatCategories() should contain 'feature'")
	}

	pris := formatPriorities()
	if pris == "" {
		t.Error("formatPriorities() returned empty string")
	}
	if !strings.Contains(pris, "normal") {
		t.Error("formatPriorities() should contain 'normal'")
	}
}
