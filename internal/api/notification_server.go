// Package api provides the Connect RPC and REST API server for orc.
// This file implements the NotificationService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/storage"
)

// notificationServer implements the NotificationServiceHandler interface.
type notificationServer struct {
	orcv1connect.UnimplementedNotificationServiceHandler
	backend storage.Backend
	logger  *slog.Logger
}

// NewNotificationServer creates a new NotificationService handler.
func NewNotificationServer(
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.NotificationServiceHandler {
	return &notificationServer{
		backend: backend,
		logger:  logger,
	}
}

// ListNotifications returns all active notifications.
func (s *notificationServer) ListNotifications(
	ctx context.Context,
	req *connect.Request[orcv1.ListNotificationsRequest],
) (*connect.Response[orcv1.ListNotificationsResponse], error) {
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database backend required for notifications"))
	}

	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	notifications, err := adapter.GetActiveNotifications(ctx)
	if err != nil {
		s.logger.Error("failed to get notifications", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get notifications: %w", err))
	}

	protoNotifications := make([]*orcv1.Notification, 0, len(notifications))
	for _, n := range notifications {
		protoNotifications = append(protoNotifications, notificationToProto(n))
	}

	return connect.NewResponse(&orcv1.ListNotificationsResponse{
		Notifications: protoNotifications,
	}), nil
}

// DismissNotification dismisses a single notification.
func (s *notificationServer) DismissNotification(
	ctx context.Context,
	req *connect.Request[orcv1.DismissNotificationRequest],
) (*connect.Response[orcv1.DismissNotificationResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notification ID required"))
	}

	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database backend required for notifications"))
	}

	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	if err := adapter.DismissNotification(ctx, req.Msg.Id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("notification not found: %s", req.Msg.Id))
		}
		s.logger.Error("failed to dismiss notification", "id", req.Msg.Id, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to dismiss notification: %w", err))
	}

	s.logger.Info("notification dismissed", "id", req.Msg.Id)
	return connect.NewResponse(&orcv1.DismissNotificationResponse{}), nil
}

// DismissAllNotifications dismisses all active notifications.
func (s *notificationServer) DismissAllNotifications(
	ctx context.Context,
	req *connect.Request[orcv1.DismissAllNotificationsRequest],
) (*connect.Response[orcv1.DismissAllNotificationsResponse], error) {
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database backend required for notifications"))
	}

	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	if err := adapter.DismissAllNotifications(ctx); err != nil {
		s.logger.Error("failed to dismiss all notifications", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to dismiss all notifications: %w", err))
	}

	s.logger.Info("all notifications dismissed")
	return connect.NewResponse(&orcv1.DismissAllNotificationsResponse{}), nil
}

// notificationToProto converts an automation.Notification to proto.
func notificationToProto(n *automation.Notification) *orcv1.Notification {
	proto := &orcv1.Notification{
		Id:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		CreatedAt: timestamppb.New(n.CreatedAt),
	}

	if n.Message != "" {
		proto.Message = &n.Message
	}
	if n.SourceType != "" {
		proto.SourceType = &n.SourceType
	}
	if n.SourceID != "" {
		proto.SourceId = &n.SourceID
	}
	if n.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*n.ExpiresAt)
	}

	return proto
}
