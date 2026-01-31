// Package initiative provides proto-based initiative helper functions.
// These functions operate on orcv1.Initiative proto types, providing the same
// functionality as the original Initiative methods but as standalone functions.
package initiative

import (
	"fmt"
	"slices"
	"sort"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewProtoInitiative creates a new proto Initiative with sensible defaults.
func NewProtoInitiative(id, title string) *orcv1.Initiative {
	now := timestamppb.Now()
	return &orcv1.Initiative{
		Version:   1,
		Id:        id,
		Title:     title,
		Status:    orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddDecisionProto records a decision in the initiative.
func AddDecisionProto(i *orcv1.Initiative, decision, rationale, by string) {
	if i == nil {
		return
	}
	id := fmt.Sprintf("DEC-%03d", len(i.Decisions)+1)
	dec := &orcv1.InitiativeDecision{
		Id:       id,
		Date:     timestamppb.Now(),
		By:       by,
		Decision: decision,
	}
	if rationale != "" {
		dec.Rationale = &rationale
	}
	i.Decisions = append(i.Decisions, dec)
	i.UpdatedAt = timestamppb.Now()
}

// IsBlockedProto returns true if any blocking initiative is not completed.
func IsBlockedProto(i *orcv1.Initiative, initiatives map[string]*orcv1.Initiative) bool {
	if i == nil {
		return false
	}
	for _, depID := range i.BlockedBy {
		dep, exists := initiatives[depID]
		if !exists {
			// Missing initiative is treated as unmet dependency
			return true
		}
		if dep.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED {
			return true
		}
	}
	return false
}

// ProtoBlockerInfo contains information about a blocking initiative for display purposes.
type ProtoBlockerInfo struct {
	ID     string                    `json:"id"`
	Title  string                    `json:"title"`
	Status orcv1.InitiativeStatus    `json:"status"`
}

// GetIncompleteBlockersProto returns full information about blocking initiatives that aren't completed.
func GetIncompleteBlockersProto(i *orcv1.Initiative, initiatives map[string]*orcv1.Initiative) []ProtoBlockerInfo {
	if i == nil {
		return nil
	}
	var blockers []ProtoBlockerInfo
	for _, blockerID := range i.BlockedBy {
		blocker, exists := initiatives[blockerID]
		if !exists {
			blockers = append(blockers, ProtoBlockerInfo{
				ID:     blockerID,
				Title:  "(initiative not found)",
				Status: orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED,
			})
			continue
		}
		if blocker.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED {
			blockers = append(blockers, ProtoBlockerInfo{
				ID:     blocker.Id,
				Title:  blocker.Title,
				Status: blocker.Status,
			})
		}
	}
	return blockers
}

// ComputeBlocksProto calculates the Blocks field for an initiative by scanning all initiatives.
func ComputeBlocksProto(initID string, allInits []*orcv1.Initiative) []string {
	var blocks []string
	for _, init := range allInits {
		if slices.Contains(init.BlockedBy, initID) {
			blocks = append(blocks, init.Id)
		}
	}
	sort.Strings(blocks)
	return blocks
}

// PopulateComputedFieldsProto fills in Blocks for all initiatives.
func PopulateComputedFieldsProto(initiatives []*orcv1.Initiative) {
	for _, init := range initiatives {
		init.Blocks = ComputeBlocksProto(init.Id, initiatives)
	}
}

// UpdateTimestampProto sets the UpdatedAt field to the current time.
func UpdateTimestampProto(i *orcv1.Initiative) {
	if i == nil {
		return
	}
	i.UpdatedAt = timestamppb.Now()
}

