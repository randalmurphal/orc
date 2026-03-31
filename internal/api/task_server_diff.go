package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/diff"
)

// GetDiff returns the diff for a task's changes.
func (s *taskServer) GetDiff(
	ctx context.Context,
	req *connect.Request[orcv1.GetDiffRequest],
) (*connect.Response[orcv1.GetDiffResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	var result *diff.DiffResult

	if t.Pr != nil && t.Pr.Merged && t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
		result, err = diffSvc.GetMergeCommitDiff(ctx, *t.Pr.MergeCommitSha)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get merge commit diff: %w", err))
		}
	} else {
		base := "main"
		head := t.Branch
		if head == "" {
			head = "HEAD"
		}

		base = diffSvc.ResolveRef(ctx, base)
		head = diffSvc.ResolveRef(ctx, head)

		useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(ctx, base, head)
		if useWorkingTree {
			head = effectiveHead
		}

		result, err = diffSvc.GetFullDiff(ctx, base, head)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get diff: %w", err))
		}
	}

	return connect.NewResponse(&orcv1.GetDiffResponse{
		Diff: diffResultToProto(result),
	}), nil
}

// GetDiffStats returns just the diff statistics.
func (s *taskServer) GetDiffStats(
	ctx context.Context,
	req *connect.Request[orcv1.GetDiffStatsRequest],
) (*connect.Response[orcv1.GetDiffStatsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	var stats *diff.DiffStats

	if t.Pr != nil && t.Pr.Merged && t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
		sha := *t.Pr.MergeCommitSha
		stats, err = diffSvc.GetStats(ctx, sha+"^", sha)
	} else {
		base := "main"
		head := t.Branch
		if head == "" {
			head = "HEAD"
		}

		base = diffSvc.ResolveRef(ctx, base)
		head = diffSvc.ResolveRef(ctx, head)

		useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(ctx, base, head)
		if useWorkingTree {
			head = effectiveHead
		}

		stats, err = diffSvc.GetStats(ctx, base, head)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get diff stats: %w", err))
	}

	return connect.NewResponse(&orcv1.GetDiffStatsResponse{
		Stats: &orcv1.DiffStats{
			FilesChanged: int32(stats.FilesChanged),
			Additions:    int32(stats.Additions),
			Deletions:    int32(stats.Deletions),
		},
	}), nil
}

// GetFileDiff returns the diff for a single file with hunks.
func (s *taskServer) GetFileDiff(
	ctx context.Context,
	req *connect.Request[orcv1.GetFileDiffRequest],
) (*connect.Response[orcv1.GetFileDiffResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.FilePath == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	var fileDiff *diff.FileDiff

	if t.Pr != nil && t.Pr.Merged && t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
		fileDiff, err = diffSvc.GetMergeCommitFileDiff(ctx, *t.Pr.MergeCommitSha, req.Msg.FilePath)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get merge commit file diff: %w", err))
		}
	} else {
		base := "main"
		head := t.Branch
		if head == "" {
			head = "HEAD"
		}

		base = diffSvc.ResolveRef(ctx, base)
		head = diffSvc.ResolveRef(ctx, head)

		useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(ctx, base, head)
		if useWorkingTree {
			head = effectiveHead
		}

		fileDiff, err = diffSvc.GetFileDiff(ctx, base, head, req.Msg.FilePath)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get file diff: %w", err))
		}
	}

	return connect.NewResponse(&orcv1.GetFileDiffResponse{
		File: fileDiffToProto(fileDiff),
	}), nil
}

func fileDiffToProto(f *diff.FileDiff) *orcv1.FileDiff {
	if f == nil {
		return nil
	}
	fileDiff := &orcv1.FileDiff{
		Path:      f.Path,
		Status:    f.Status,
		Additions: int32(f.Additions),
		Deletions: int32(f.Deletions),
		Binary:    f.Binary,
		Syntax:    f.Syntax,
	}
	if f.OldPath != "" {
		fileDiff.OldPath = &f.OldPath
	}
	fileDiff.Hunks = make([]*orcv1.DiffHunk, len(f.Hunks))
	for j, h := range f.Hunks {
		fileDiff.Hunks[j] = &orcv1.DiffHunk{
			OldStart: int32(h.OldStart),
			OldLines: int32(h.OldLines),
			NewStart: int32(h.NewStart),
			NewLines: int32(h.NewLines),
		}
		fileDiff.Hunks[j].Lines = make([]*orcv1.DiffLine, len(h.Lines))
		for k, l := range h.Lines {
			line := &orcv1.DiffLine{
				Type:    l.Type,
				Content: l.Content,
			}
			if l.OldLine > 0 {
				oldLine := int32(l.OldLine)
				line.OldLine = &oldLine
			}
			if l.NewLine > 0 {
				newLine := int32(l.NewLine)
				line.NewLine = &newLine
			}
			fileDiff.Hunks[j].Lines[k] = line
		}
	}
	return fileDiff
}

func diffResultToProto(d *diff.DiffResult) *orcv1.DiffResult {
	if d == nil {
		return nil
	}
	result := &orcv1.DiffResult{
		Base: d.Base,
		Head: d.Head,
		Stats: &orcv1.DiffStats{
			FilesChanged: int32(d.Stats.FilesChanged),
			Additions:    int32(d.Stats.Additions),
			Deletions:    int32(d.Stats.Deletions),
		},
	}

	result.Files = make([]*orcv1.FileDiff, len(d.Files))
	for i, f := range d.Files {
		fileDiff := &orcv1.FileDiff{
			Path:      f.Path,
			Status:    f.Status,
			Additions: int32(f.Additions),
			Deletions: int32(f.Deletions),
			Binary:    f.Binary,
			Syntax:    f.Syntax,
		}
		if f.OldPath != "" {
			fileDiff.OldPath = &f.OldPath
		}
		fileDiff.Hunks = make([]*orcv1.DiffHunk, len(f.Hunks))
		for j, h := range f.Hunks {
			fileDiff.Hunks[j] = &orcv1.DiffHunk{
				OldStart: int32(h.OldStart),
				OldLines: int32(h.OldLines),
				NewStart: int32(h.NewStart),
				NewLines: int32(h.NewLines),
			}
			fileDiff.Hunks[j].Lines = make([]*orcv1.DiffLine, len(h.Lines))
			for k, l := range h.Lines {
				line := &orcv1.DiffLine{
					Type:    l.Type,
					Content: l.Content,
				}
				if l.OldLine > 0 {
					oldLine := int32(l.OldLine)
					line.OldLine = &oldLine
				}
				if l.NewLine > 0 {
					newLine := int32(l.NewLine)
					line.NewLine = &newLine
				}
				fileDiff.Hunks[j].Lines[k] = line
			}
		}
		result.Files[i] = fileDiff
	}

	return result
}
