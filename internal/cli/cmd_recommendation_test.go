package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestRecommendationCommand_List(t *testing.T) {
	originalFactory := recommendationCLIClientFactory
	recommendationCLIClientFactory = func() (recommendationCLIClient, string, error) {
		return &stubRecommendationClient{
			listResponse: &orcv1.ListRecommendationsResponse{
				Recommendations: []*orcv1.Recommendation{
					{
						Id:           "REC-001",
						Status:       orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
						Kind:         orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
						SourceTaskId: "TASK-001",
						Title:        "Clean up duplicate polling",
					},
				},
			},
		}, "", nil
	}
	t.Cleanup(func() { recommendationCLIClientFactory = originalFactory })

	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"list"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), "REC-001")
	require.Contains(t, stdout.String(), "Clean up duplicate polling")
}

func TestRecommendationCommand_Accept(t *testing.T) {
	originalFactory := recommendationCLIClientFactory
	recommendationCLIClientFactory = func() (recommendationCLIClient, string, error) {
		return &stubRecommendationClient{
			acceptResponse: &orcv1.AcceptRecommendationResponse{
				Recommendation: &orcv1.Recommendation{
					Id:             "REC-002",
					Title:          "Investigate flaky tests",
					PromotedToType: "task",
					PromotedToId:   "TASK-123",
				},
			},
		}, "", nil
	}
	t.Cleanup(func() { recommendationCLIClientFactory = originalFactory })

	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"accept", "REC-002", "--by", "randy", "--reason", "worth doing"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), "Accepted REC-002")
	require.Contains(t, stdout.String(), "task TASK-123")
}

func TestRecommendationCommand_Reject(t *testing.T) {
	originalFactory := recommendationCLIClientFactory
	recommendationCLIClientFactory = func() (recommendationCLIClient, string, error) {
		return &stubRecommendationClient{
			rejectResponse: &orcv1.RejectRecommendationResponse{
				Recommendation: &orcv1.Recommendation{
					Id:    "REC-003",
					Title: "Skip this cleanup",
				},
			},
		}, "", nil
	}
	t.Cleanup(func() { recommendationCLIClientFactory = originalFactory })

	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"reject", "REC-003", "--by", "randy", "--reason", "not worth it"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), "Rejected REC-003")
}

func TestRecommendationCommand_Discuss(t *testing.T) {
	originalFactory := recommendationCLIClientFactory
	recommendationCLIClientFactory = func() (recommendationCLIClient, string, error) {
		return &stubRecommendationClient{
			discussResponse: &orcv1.DiscussRecommendationResponse{
				Recommendation: &orcv1.Recommendation{
					Id:    "REC-004",
					Title: "Discuss rollout",
				},
				ContextPack: "Recommendation REC-004\nKind: risk",
			},
		}, "", nil
	}
	t.Cleanup(func() { recommendationCLIClientFactory = originalFactory })

	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"discuss", "REC-004", "--by", "randy", "--reason", "needs a thread"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), "Discussed REC-004")
	require.Contains(t, stdout.String(), "Recommendation REC-004")
}

func TestRecommendationCommand_HelpIncludesSubcommands(t *testing.T) {
	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), "list")
	require.Contains(t, stdout.String(), "accept")
	require.Contains(t, stdout.String(), "reject")
	require.Contains(t, stdout.String(), "discuss")
}

func TestRecommendationCommand_ListRejectsInvalidStatusFilter(t *testing.T) {
	originalFactory := recommendationCLIClientFactory
	recommendationCLIClientFactory = func() (recommendationCLIClient, string, error) {
		return &stubRecommendationClient{}, "", nil
	}
	t.Cleanup(func() { recommendationCLIClientFactory = originalFactory })

	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"list", "--status", "pendng"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid recommendation status")
}

func TestRecommendationCommand_ListRejectsInvalidKindFilter(t *testing.T) {
	originalFactory := recommendationCLIClientFactory
	recommendationCLIClientFactory = func() (recommendationCLIClient, string, error) {
		return &stubRecommendationClient{}, "", nil
	}
	t.Cleanup(func() { recommendationCLIClientFactory = originalFactory })

	cmd := newRecommendationCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"list", "--kind", "cleanup-ish"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid recommendation kind")
}

type stubRecommendationClient struct {
	listResponse    *orcv1.ListRecommendationsResponse
	acceptResponse  *orcv1.AcceptRecommendationResponse
	rejectResponse  *orcv1.RejectRecommendationResponse
	discussResponse *orcv1.DiscussRecommendationResponse
}

func (s *stubRecommendationClient) ListRecommendations(ctx context.Context, req *orcv1.ListRecommendationsRequest) (*orcv1.ListRecommendationsResponse, error) {
	return s.listResponse, nil
}

func (s *stubRecommendationClient) AcceptRecommendation(ctx context.Context, req *orcv1.AcceptRecommendationRequest) (*orcv1.AcceptRecommendationResponse, error) {
	return s.acceptResponse, nil
}

func (s *stubRecommendationClient) RejectRecommendation(ctx context.Context, req *orcv1.RejectRecommendationRequest) (*orcv1.RejectRecommendationResponse, error) {
	return s.rejectResponse, nil
}

func (s *stubRecommendationClient) DiscussRecommendation(ctx context.Context, req *orcv1.DiscussRecommendationRequest) (*orcv1.DiscussRecommendationResponse, error) {
	return s.discussResponse, nil
}
