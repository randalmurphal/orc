package decision

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// NewProtoDecision creates a new proto Decision with default values.
func NewProtoDecision(id, taskID, question string) *orcv1.PendingDecision {
	return &orcv1.PendingDecision{
		Id:          id,
		TaskId:      taskID,
		Question:    question,
		RequestedAt: timestamppb.New(time.Now()),
		Options:     []*orcv1.DecisionOption{},
	}
}