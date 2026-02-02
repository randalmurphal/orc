package gate

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// NewProtoGateApproval creates a new proto AttentionItem representing a gate approval.
func NewProtoGateApproval(id, taskID, phase string) *orcv1.AttentionItem {
	return &orcv1.AttentionItem{
		Id:        id,
		Type:      orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_GATE_APPROVAL,
		TaskId:    taskID,
		Title:     "Gate approval for " + phase,
		CreatedAt: timestamppb.New(time.Now()),
		AvailableActions: []orcv1.AttentionAction{
			orcv1.AttentionAction_ATTENTION_ACTION_APPROVE,
			orcv1.AttentionAction_ATTENTION_ACTION_REJECT,
		},
	}
}