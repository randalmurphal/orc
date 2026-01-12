package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaming_BranchName(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		executorPrefix string
		want           string
	}{
		{
			name:           "solo mode - no prefix",
			taskID:         "TASK-001",
			executorPrefix: "",
			want:           "orc/TASK-001",
		},
		{
			name:           "p2p mode - with prefix",
			taskID:         "TASK-001",
			executorPrefix: "am",
			want:           "orc/TASK-001-am",
		},
		{
			name:           "p2p mode - uppercase prefix normalized",
			taskID:         "TASK-001",
			executorPrefix: "AM",
			want:           "orc/TASK-001-am",
		},
		{
			name:           "prefixed task ID with executor prefix",
			taskID:         "TASK-AM-001",
			executorPrefix: "bj",
			want:           "orc/TASK-AM-001-bj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.taskID, tt.executorPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorktreeDirName(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		executorPrefix string
		want           string
	}{
		{
			name:           "solo mode - no prefix",
			taskID:         "TASK-001",
			executorPrefix: "",
			want:           "orc-TASK-001",
		},
		{
			name:           "p2p mode - with prefix",
			taskID:         "TASK-001",
			executorPrefix: "am",
			want:           "orc-TASK-001-am",
		},
		{
			name:           "prefixed task ID with executor prefix",
			taskID:         "TASK-AM-001",
			executorPrefix: "bj",
			want:           "orc-TASK-AM-001-bj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreeDirName(tt.taskID, tt.executorPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNaming_WorktreePath(t *testing.T) {
	tests := []struct {
		name           string
		worktreeDir    string
		taskID         string
		executorPrefix string
		want           string
	}{
		{
			name:           "solo mode",
			worktreeDir:    ".orc/worktrees",
			taskID:         "TASK-001",
			executorPrefix: "",
			want:           ".orc/worktrees/orc-TASK-001",
		},
		{
			name:           "p2p mode",
			worktreeDir:    ".orc/worktrees",
			taskID:         "TASK-001",
			executorPrefix: "am",
			want:           ".orc/worktrees/orc-TASK-001-am",
		},
		{
			name:           "absolute path",
			worktreeDir:    "/home/user/project/.orc/worktrees",
			taskID:         "TASK-AM-001",
			executorPrefix: "bj",
			want:           "/home/user/project/.orc/worktrees/orc-TASK-AM-001-bj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreePath(tt.worktreeDir, tt.taskID, tt.executorPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseBranchName(t *testing.T) {
	tests := []struct {
		name         string
		branch       string
		wantTaskID   string
		wantExecutor string
		wantOK       bool
	}{
		{
			name:         "solo mode branch",
			branch:       "orc/TASK-001",
			wantTaskID:   "TASK-001",
			wantExecutor: "",
			wantOK:       true,
		},
		{
			name:         "p2p mode branch with executor",
			branch:       "orc/TASK-001-am",
			wantTaskID:   "TASK-001",
			wantExecutor: "am",
			wantOK:       true,
		},
		{
			name:         "prefixed task with executor",
			branch:       "orc/TASK-AM-001-bj",
			wantTaskID:   "TASK-AM-001",
			wantExecutor: "bj",
			wantOK:       true,
		},
		{
			name:         "three-letter executor prefix",
			branch:       "orc/TASK-001-abc",
			wantTaskID:   "TASK-001",
			wantExecutor: "abc",
			wantOK:       true,
		},
		{
			name:         "non-orc branch",
			branch:       "main",
			wantTaskID:   "",
			wantExecutor: "",
			wantOK:       false,
		},
		{
			name:         "feature branch",
			branch:       "feature/something",
			wantTaskID:   "",
			wantExecutor: "",
			wantOK:       false,
		},
		{
			name:         "prefixed task without executor",
			branch:       "orc/TASK-AM-001",
			wantTaskID:   "TASK-AM-001",
			wantExecutor: "",
			wantOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID, executor, ok := ParseBranchName(tt.branch)
			assert.Equal(t, tt.wantOK, ok, "ok mismatch")
			if tt.wantOK {
				assert.Equal(t, tt.wantTaskID, taskID, "taskID mismatch")
				assert.Equal(t, tt.wantExecutor, executor, "executor mismatch")
			}
		})
	}
}
