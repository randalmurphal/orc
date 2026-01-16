package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
		errMsg  string
	}{
		// Valid branch names
		{
			name:    "simple branch name",
			branch:  "main",
			wantErr: false,
		},
		{
			name:    "branch with slash",
			branch:  "feature/auth",
			wantErr: false,
		},
		{
			name:    "branch with hyphen",
			branch:  "feature-auth",
			wantErr: false,
		},
		{
			name:    "branch with underscore",
			branch:  "feature_auth",
			wantErr: false,
		},
		{
			name:    "branch with dot",
			branch:  "v1.0.0",
			wantErr: false,
		},
		{
			name:    "orc task branch",
			branch:  "orc/TASK-001",
			wantErr: false,
		},
		{
			name:    "initiative prefix branch",
			branch:  "feature/auth-TASK-001",
			wantErr: false,
		},
		{
			name:    "developer staging branch",
			branch:  "dev/randy",
			wantErr: false,
		},
		{
			name:    "deep nested branch",
			branch:  "team/project/feature/auth",
			wantErr: false,
		},
		// Invalid branch names - security issues
		{
			name:    "empty branch name",
			branch:  "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "path traversal",
			branch:  "../../../etc/passwd",
			wantErr: true,
			errMsg:  "cannot contain '..'",
		},
		{
			name:    "path traversal in middle",
			branch:  "feature/../../../etc/passwd",
			wantErr: true,
			errMsg:  "cannot contain '..'",
		},
		{
			name:    "shell metacharacter semicolon",
			branch:  "main;rm",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "shell metacharacter pipe",
			branch:  "main|cat",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "shell metacharacter ampersand",
			branch:  "main&&rm",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "shell metacharacter backtick",
			branch:  "main`rm`",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "shell metacharacter dollar",
			branch:  "main$(rm)",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "newline injection",
			branch:  "main\nrm",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "space in branch name",
			branch:  "my branch",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		// Invalid branch names - git restrictions
		{
			name:    "ends with .lock",
			branch:  "branch.lock",
			wantErr: true,
			errMsg:  "cannot end with '.lock'",
		},
		{
			name:    "ends with slash",
			branch:  "feature/",
			wantErr: true,
			errMsg:  "cannot end with '/'",
		},
		{
			name:    "double slash",
			branch:  "feature//auth",
			wantErr: true,
			errMsg:  "cannot contain '//'",
		},
		{
			name:    "starts with hyphen",
			branch:  "-feature",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "starts with dot",
			branch:  ".hidden",
			wantErr: true,
			errMsg:  "contains invalid characters",
		},
		{
			name:    "exceeds max length",
			branch:  strings.Repeat("a", 257),
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name:    "at max length",
			branch:  strings.Repeat("a", 256),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branch)
			if tt.wantErr {
				assert.Error(t, err, "expected error for branch: %s", tt.branch)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message mismatch")
				}
			} else {
				assert.NoError(t, err, "unexpected error for branch: %s", tt.branch)
			}
		})
	}
}

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

func TestBranchNameWithPrefix(t *testing.T) {
	tests := []struct {
		name             string
		taskID           string
		executorPrefix   string
		initiativePrefix string
		want             string
	}{
		{
			name:             "no initiative prefix - solo mode",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "",
			want:             "orc/TASK-001",
		},
		{
			name:             "no initiative prefix - with executor",
			taskID:           "TASK-001",
			executorPrefix:   "am",
			initiativePrefix: "",
			want:             "orc/TASK-001-am",
		},
		{
			name:             "initiative prefix with slash - solo mode",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "feature/auth-",
			want:             "feature/auth-TASK-001",
		},
		{
			name:             "initiative prefix with slash - with executor",
			taskID:           "TASK-001",
			executorPrefix:   "am",
			initiativePrefix: "feature/auth-",
			want:             "feature/auth-TASK-001-am",
		},
		{
			name:             "initiative prefix no trailing separator",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "feature/payments",
			want:             "feature/paymentsTASK-001",
		},
		{
			name:             "simple initiative prefix",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "auth-",
			want:             "auth-TASK-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchNameWithPrefix(tt.taskID, tt.executorPrefix, tt.initiativePrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorktreeDirNameWithPrefix(t *testing.T) {
	tests := []struct {
		name             string
		taskID           string
		executorPrefix   string
		initiativePrefix string
		want             string
	}{
		{
			name:             "no initiative prefix",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "",
			want:             "orc-TASK-001",
		},
		{
			name:             "initiative prefix with slash converted to hyphen",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "feature/auth-",
			want:             "feature-auth-TASK-001",
		},
		{
			name:             "initiative prefix with slash and executor",
			taskID:           "TASK-001",
			executorPrefix:   "am",
			initiativePrefix: "feature/auth-",
			want:             "feature-auth-TASK-001-am",
		},
		{
			name:             "deep nested initiative prefix",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "team/project/feature-",
			want:             "team-project-feature-TASK-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreeDirNameWithPrefix(tt.taskID, tt.executorPrefix, tt.initiativePrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorkTreePathWithPrefix(t *testing.T) {
	tests := []struct {
		name             string
		worktreeDir      string
		taskID           string
		executorPrefix   string
		initiativePrefix string
		want             string
	}{
		{
			name:             "no initiative prefix",
			worktreeDir:      ".orc/worktrees",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "",
			want:             ".orc/worktrees/orc-TASK-001",
		},
		{
			name:             "initiative prefix converted for path",
			worktreeDir:      ".orc/worktrees",
			taskID:           "TASK-001",
			executorPrefix:   "",
			initiativePrefix: "feature/auth-",
			want:             ".orc/worktrees/feature-auth-TASK-001",
		},
		{
			name:             "absolute path with initiative prefix",
			worktreeDir:      "/home/user/project/.orc/worktrees",
			taskID:           "TASK-001",
			executorPrefix:   "am",
			initiativePrefix: "feature/auth-",
			want:             "/home/user/project/.orc/worktrees/feature-auth-TASK-001-am",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreePathWithPrefix(tt.worktreeDir, tt.taskID, tt.executorPrefix, tt.initiativePrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}
