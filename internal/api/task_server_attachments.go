package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// ListAttachments returns all attachments for a task.
func (s *taskServer) ListAttachments(
	ctx context.Context,
	req *connect.Request[orcv1.ListAttachmentsRequest],
) (*connect.Response[orcv1.ListAttachmentsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	attachments, err := backend.ListAttachments(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list attachments: %w", err))
	}

	protoAttachments := make([]*orcv1.Attachment, len(attachments))
	for i, a := range attachments {
		protoAttachments[i] = attachmentToProto(a)
	}

	return connect.NewResponse(&orcv1.ListAttachmentsResponse{
		Attachments: protoAttachments,
	}), nil
}

// UploadAttachment uploads a file attachment (client streaming).
func (s *taskServer) UploadAttachment(
	ctx context.Context,
	stream *connect.ClientStream[orcv1.UploadAttachmentRequest],
) (*connect.Response[orcv1.UploadAttachmentResponse], error) {
	var taskID, filename, contentType, projectID string
	var data []byte

	for stream.Receive() {
		msg := stream.Msg()
		switch d := msg.Data.(type) {
		case *orcv1.UploadAttachmentRequest_Metadata:
			taskID = d.Metadata.TaskId
			filename = d.Metadata.Filename
			contentType = d.Metadata.ContentType
			projectID = d.Metadata.ProjectId
		case *orcv1.UploadAttachmentRequest_Chunk:
			data = append(data, d.Chunk...)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("stream error: %w", err))
	}

	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if filename == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("filename is required"))
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	attachment, err := backend.SaveAttachment(taskID, filename, contentType, data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save attachment: %w", err))
	}

	return connect.NewResponse(&orcv1.UploadAttachmentResponse{
		Attachment: attachmentToProto(attachment),
	}), nil
}

// DownloadAttachment downloads a file attachment (server streaming).
func (s *taskServer) DownloadAttachment(
	ctx context.Context,
	req *connect.Request[orcv1.DownloadAttachmentRequest],
	stream *connect.ServerStream[orcv1.DownloadAttachmentResponse],
) error {
	if req.Msg.TaskId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Filename == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("filename is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	_, data, err := backend.GetAttachment(req.Msg.TaskId, req.Msg.Filename)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("attachment not found: %w", err))
	}

	chunkSize := 64 * 1024
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&orcv1.DownloadAttachmentResponse{
			Chunk: data[i:end],
		}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteAttachment deletes a file attachment.
func (s *taskServer) DeleteAttachment(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteAttachmentRequest],
) (*connect.Response[orcv1.DeleteAttachmentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Filename == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("filename is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	if err := backend.DeleteAttachment(req.Msg.TaskId, req.Msg.Filename); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete attachment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteAttachmentResponse{
		Message: "Attachment deleted",
	}), nil
}

func attachmentToProto(a *task.Attachment) *orcv1.Attachment {
	if a == nil {
		return nil
	}
	return &orcv1.Attachment{
		Filename:    a.Filename,
		Size:        a.Size,
		ContentType: a.ContentType,
		CreatedAt:   timestamppb.New(a.CreatedAt),
		IsImage:     a.IsImage,
	}
}
