package task

import (
	"encoding/json"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

const executorDiagnosticMetadataKey = "_executor_diagnostic"

type ExecutorDiagnostic struct {
	Kind          string `json:"kind"`
	Phase         string `json:"phase,omitempty"`
	Reason        string `json:"reason"`
	Detail        string `json:"detail,omitempty"`
	ExecutorPID   int32  `json:"executor_pid,omitempty"`
	DetectedAt    string `json:"detected_at,omitempty"`
	LastHeartbeat string `json:"last_heartbeat,omitempty"`
}

func SetExecutorDiagnosticProto(t *orcv1.Task, diagnostic ExecutorDiagnostic) {
	if t == nil {
		return
	}
	EnsureMetadataProto(t)
	data, err := json.Marshal(diagnostic)
	if err != nil {
		return
	}
	t.Metadata[executorDiagnosticMetadataKey] = string(data)
}

func GetExecutorDiagnosticProto(t *orcv1.Task) *ExecutorDiagnostic {
	if t == nil || t.Metadata == nil {
		return nil
	}
	raw := t.Metadata[executorDiagnosticMetadataKey]
	if raw == "" {
		return nil
	}
	var diagnostic ExecutorDiagnostic
	if err := json.Unmarshal([]byte(raw), &diagnostic); err != nil {
		return nil
	}
	return &diagnostic
}

func ClearExecutorDiagnosticProto(t *orcv1.Task) {
	if t == nil || t.Metadata == nil {
		return
	}
	delete(t.Metadata, executorDiagnosticMetadataKey)
}
