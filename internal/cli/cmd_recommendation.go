package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
)

type recommendationCLIClient interface {
	ListRecommendations(ctx context.Context, req *orcv1.ListRecommendationsRequest) (*orcv1.ListRecommendationsResponse, error)
	AcceptRecommendation(ctx context.Context, req *orcv1.AcceptRecommendationRequest) (*orcv1.AcceptRecommendationResponse, error)
	RejectRecommendation(ctx context.Context, req *orcv1.RejectRecommendationRequest) (*orcv1.RejectRecommendationResponse, error)
	DiscussRecommendation(ctx context.Context, req *orcv1.DiscussRecommendationRequest) (*orcv1.DiscussRecommendationResponse, error)
}

var recommendationCLIClientFactory = newRecommendationCLIClient

func newRecommendationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recommendation",
		Short: "Manage project recommendations",
		Long: `Manage project recommendations that require explicit human decisions.

Recommendations stay out of the real backlog until a human accepts, rejects,
or discusses them through the recommendation inbox.`,
	}

	cmd.AddCommand(newRecommendationListCmd())
	cmd.AddCommand(newRecommendationAcceptCmd())
	cmd.AddCommand(newRecommendationRejectCmd())
	cmd.AddCommand(newRecommendationDiscussCmd())

	return cmd
}

func newRecommendationListCmd() *cobra.Command {
	var status string
	var kind string
	var sourceTaskID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recommendations for the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, projectID, err := recommendationCLIClientFactory()
			if err != nil {
				return err
			}

			statusProto, err := parseRecommendationStatusFlag(status)
			if err != nil {
				return err
			}
			kindProto, err := parseRecommendationKindFlag(kind)
			if err != nil {
				return err
			}

			req := &orcv1.ListRecommendationsRequest{
				ProjectId:    projectID,
				Status:       statusProto,
				Kind:         kindProto,
				SourceTaskId: sourceTaskID,
			}
			resp, err := client.ListRecommendations(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("list recommendations: %w", err)
			}

			if jsonOut {
				data, err := json.MarshalIndent(resp.Recommendations, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal recommendations: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			if len(resp.Recommendations) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No recommendations found")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSTATUS\tKIND\tTASK\tTITLE")
			for _, rec := range resp.Recommendations {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					rec.Id,
					recommendationStatusLabel(rec.Status),
					recommendationKindLabel(rec.Kind),
					rec.SourceTaskId,
					rec.Title,
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status: pending, accepted, rejected, discussed")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind: cleanup, risk, follow-up, decision-request")
	cmd.Flags().StringVar(&sourceTaskID, "source-task", "", "Filter by source task ID")

	return cmd
}

func newRecommendationAcceptCmd() *cobra.Command {
	return newRecommendationDecisionCmd(
		"accept",
		"Accept a recommendation",
		func(ctx context.Context, client recommendationCLIClient, req *recommendationDecisionRequest) (string, error) {
			resp, err := client.AcceptRecommendation(ctx, &orcv1.AcceptRecommendationRequest{
				ProjectId:        req.projectID,
				RecommendationId: req.recommendationID,
				DecidedBy:        req.decidedBy,
				DecisionReason:   req.decisionReason,
			})
			if err != nil {
				return "", err
			}
			return formatRecommendationDecision("accepted", resp.Recommendation), nil
		},
	)
}

func newRecommendationRejectCmd() *cobra.Command {
	return newRecommendationDecisionCmd(
		"reject",
		"Reject a recommendation",
		func(ctx context.Context, client recommendationCLIClient, req *recommendationDecisionRequest) (string, error) {
			resp, err := client.RejectRecommendation(ctx, &orcv1.RejectRecommendationRequest{
				ProjectId:        req.projectID,
				RecommendationId: req.recommendationID,
				DecidedBy:        req.decidedBy,
				DecisionReason:   req.decisionReason,
			})
			if err != nil {
				return "", err
			}
			return formatRecommendationDecision("rejected", resp.Recommendation), nil
		},
	)
}

func newRecommendationDiscussCmd() *cobra.Command {
	return newRecommendationDecisionCmd(
		"discuss",
		"Mark a recommendation for discussion and print its context pack",
		func(ctx context.Context, client recommendationCLIClient, req *recommendationDecisionRequest) (string, error) {
			resp, err := client.DiscussRecommendation(ctx, &orcv1.DiscussRecommendationRequest{
				ProjectId:        req.projectID,
				RecommendationId: req.recommendationID,
				DecidedBy:        req.decidedBy,
				DecisionReason:   req.decisionReason,
			})
			if err != nil {
				return "", err
			}
			return formatRecommendationDecision("discussed", resp.Recommendation) + "\n\n" + resp.ContextPack, nil
		},
	)
}

type recommendationDecisionRequest struct {
	projectID        string
	recommendationID string
	decidedBy        string
	decisionReason   string
}

func newRecommendationDecisionCmd(
	use string,
	short string,
	run func(ctx context.Context, client recommendationCLIClient, req *recommendationDecisionRequest) (string, error),
) *cobra.Command {
	var decidedBy string
	var decisionReason string

	cmd := &cobra.Command{
		Use:   use + " <recommendation-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, projectID, err := recommendationCLIClientFactory()
			if err != nil {
				return err
			}

			output, err := run(cmd.Context(), client, &recommendationDecisionRequest{
				projectID:        projectID,
				recommendationID: args[0],
				decidedBy:        defaultDecisionActor(decidedBy),
				decisionReason:   decisionReason,
			})
			if err != nil {
				return fmt.Errorf("%s recommendation: %w", use, err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), output)
			return nil
		},
	}

	cmd.Flags().StringVar(&decidedBy, "by", "", "Decision actor (defaults to current user)")
	cmd.Flags().StringVar(&decisionReason, "reason", "", "Decision rationale")
	return cmd
}

func newRecommendationCLIClient() (recommendationCLIClient, string, error) {
	projectID, err := ResolveProjectID()
	if err != nil {
		return nil, "", err
	}

	projectPath, err := ResolveProjectPath()
	if err != nil {
		return nil, "", err
	}

	cfg, err := config.LoadFrom(projectPath)
	if err != nil {
		cfg = config.Default()
	}

	host := cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	client := &recommendationConnectClient{
		client: orcv1connect.NewRecommendationServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", host, port)),
	}
	return client, projectID, nil
}

type recommendationConnectClient struct {
	client orcv1connect.RecommendationServiceClient
}

func (c *recommendationConnectClient) ListRecommendations(ctx context.Context, req *orcv1.ListRecommendationsRequest) (*orcv1.ListRecommendationsResponse, error) {
	resp, err := c.client.ListRecommendations(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *recommendationConnectClient) AcceptRecommendation(ctx context.Context, req *orcv1.AcceptRecommendationRequest) (*orcv1.AcceptRecommendationResponse, error) {
	resp, err := c.client.AcceptRecommendation(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *recommendationConnectClient) RejectRecommendation(ctx context.Context, req *orcv1.RejectRecommendationRequest) (*orcv1.RejectRecommendationResponse, error) {
	resp, err := c.client.RejectRecommendation(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (c *recommendationConnectClient) DiscussRecommendation(ctx context.Context, req *orcv1.DiscussRecommendationRequest) (*orcv1.DiscussRecommendationResponse, error) {
	resp, err := c.client.DiscussRecommendation(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func defaultDecisionActor(value string) string {
	if value != "" {
		return value
	}
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}
	if envUser := os.Getenv("USER"); envUser != "" {
		return envUser
	}
	return "human"
}

func recommendationStatusFlagToProto(value string) orcv1.RecommendationStatus {
	switch strings.ToLower(value) {
	case "pending":
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING
	case "accepted":
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED
	case "rejected":
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_REJECTED
	case "discussed":
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_DISCUSSED
	default:
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED
	}
}

func parseRecommendationStatusFlag(value string) (orcv1.RecommendationStatus, error) {
	status := recommendationStatusFlagToProto(value)
	if strings.TrimSpace(value) == "" || status != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED {
		return status, nil
	}
	return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, fmt.Errorf(
		"invalid recommendation status %q (expected one of: pending, accepted, rejected, discussed)",
		value,
	)
}

func recommendationKindFlagToProto(value string) orcv1.RecommendationKind {
	switch strings.ToLower(value) {
	case "cleanup":
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP
	case "risk":
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK
	case "follow-up", "follow_up":
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP
	case "decision-request", "decision_request":
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST
	default:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED
	}
}

func parseRecommendationKindFlag(value string) (orcv1.RecommendationKind, error) {
	kind := recommendationKindFlagToProto(value)
	if strings.TrimSpace(value) == "" || kind != orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED {
		return kind, nil
	}
	return orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED, fmt.Errorf(
		"invalid recommendation kind %q (expected one of: cleanup, risk, follow-up, decision-request)",
		value,
	)
}

func recommendationStatusLabel(status orcv1.RecommendationStatus) string {
	return strings.TrimPrefix(strings.ToLower(status.String()), "recommendation_status_")
}

func recommendationKindLabel(kind orcv1.RecommendationKind) string {
	return strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(kind.String()), "recommendation_kind_"), "_", "-")
}

func formatRecommendationDecision(action string, recommendation *orcv1.Recommendation) string {
	if recommendation == nil {
		return fmt.Sprintf("Recommendation %s", action)
	}
	message := fmt.Sprintf("%s %s: %s", strings.ToUpper(action[:1])+action[1:], recommendation.Id, recommendation.Title)
	if recommendation.PromotedToType != "" && recommendation.PromotedToId != "" {
		message += fmt.Sprintf(" -> %s %s", recommendation.PromotedToType, recommendation.PromotedToId)
	}
	return message
}
