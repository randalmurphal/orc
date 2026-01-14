#!/usr/bin/env python3
"""
Worktree Isolation Hook for Claude Code

This PreToolUse hook enforces that file operations stay within the worktree directory.
It blocks Edit, Write, Read, Glob, and Grep operations that target paths outside the
worktree, preventing accidental modification of the main repository.

Exit codes:
- 0: Allow tool execution
- 2: Block tool execution (stderr message shown to user)
"""

import json
import os
import sys


def get_worktree_path():
    """Get the worktree path from environment or CWD."""
    worktree_path = os.environ.get("ORC_WORKTREE_PATH")
    if worktree_path:
        return os.path.realpath(worktree_path)
    return os.path.realpath(os.getcwd())


def get_main_repo_path():
    """Get the main repository path (to explicitly block it)."""
    return os.environ.get("ORC_MAIN_REPO_PATH", "")


def normalize_path(path, worktree_path):
    """Normalize a path to an absolute path."""
    if not path:
        return None
    if not os.path.isabs(path):
        return os.path.realpath(os.path.join(worktree_path, path))
    return os.path.realpath(path)


def is_path_in_worktree(path, worktree_path):
    """Check if a path is within the worktree directory."""
    if not path:
        return True
    normalized = normalize_path(path, worktree_path)
    if not normalized:
        return True
    return normalized.startswith(worktree_path + os.sep) or normalized == worktree_path


def is_path_in_main_repo(path, worktree_path, main_repo_path):
    """Check if a path is within the main repository."""
    if not path or not main_repo_path:
        return False
    normalized = normalize_path(path, worktree_path)
    if not normalized:
        return False
    return normalized.startswith(main_repo_path + os.sep) or normalized == main_repo_path


def extract_paths_from_input(tool_name, tool_input):
    """Extract file paths from tool input based on tool type."""
    paths = []
    if tool_name in ("Edit", "Write", "Read"):
        path = tool_input.get("file_path")
        if path:
            paths.append(path)
    elif tool_name == "Glob":
        path = tool_input.get("path")
        if path:
            paths.append(path)
        pattern = tool_input.get("pattern", "")
        if pattern.startswith("/"):
            paths.append(pattern)
    elif tool_name == "Grep":
        path = tool_input.get("path")
        if path:
            paths.append(path)
    elif tool_name == "MultiEdit":
        for edit in tool_input.get("edits", []):
            path = edit.get("file_path")
            if path:
                paths.append(path)
    return paths


def main():
    try:
        input_data = json.loads(sys.stdin.read())
    except json.JSONDecodeError:
        sys.exit(0)

    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})

    file_tools = {"Edit", "Write", "Read", "Glob", "Grep", "MultiEdit"}
    if tool_name not in file_tools:
        sys.exit(0)

    worktree_path = get_worktree_path()
    main_repo_path = get_main_repo_path()
    paths = extract_paths_from_input(tool_name, tool_input)

    for path in paths:
        # IMPORTANT: Check worktree first! The worktree is inside the main repo directory,
        # so we must check if path is in worktree before checking main repo.
        if is_path_in_worktree(path, worktree_path):
            # Path is in worktree - allow it
            continue

        # Path is NOT in worktree - check if it's trying to access main repo
        if main_repo_path and is_path_in_main_repo(path, worktree_path, main_repo_path):
            print(f"""
╔══════════════════════════════════════════════════════════════════════════════╗
║  BLOCKED: File operation targeting main repository                           ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  Tool: {tool_name:<70} ║
║  Path: {path[:70]:<70} ║
║                                                                              ║
║  You are in an ISOLATED WORKTREE. Use relative paths to stay within it.      ║
║  Worktree: {worktree_path[:66]:<66} ║
╚══════════════════════════════════════════════════════════════════════════════╝
""", file=sys.stderr)
            sys.exit(2)

        # Path is outside both worktree and main repo
        print(f"""
╔══════════════════════════════════════════════════════════════════════════════╗
║  BLOCKED: File operation outside worktree                                    ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  Tool: {tool_name:<70} ║
║  Path: {path[:70]:<70} ║
║                                                                              ║
║  You are in an ISOLATED WORKTREE. Use relative paths to stay within it.      ║
║  Worktree: {worktree_path[:66]:<66} ║
╚══════════════════════════════════════════════════════════════════════════════╝
""", file=sys.stderr)
        sys.exit(2)

    sys.exit(0)


if __name__ == "__main__":
    main()
