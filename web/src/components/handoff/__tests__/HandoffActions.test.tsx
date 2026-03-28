import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { HandoffActions } from "../HandoffActions";
import {
  GenerateHandoffResponseSchema,
  HandoffSourceType,
  HandoffTarget,
} from "@/gen/orc/v1/handoff_pb";

vi.mock("@radix-ui/react-dropdown-menu", () => ({
  Root: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Trigger: ({
    children,
    ...props
  }: ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" {...props}>
      {children}
    </button>
  ),
  Portal: ({ children }: { children: ReactNode }) => <>{children}</>,
  Content: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Label: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Item: ({
    children,
    onSelect,
    ...props
  }: ButtonHTMLAttributes<HTMLButtonElement> & { onSelect?: () => void }) => (
    <button type="button" {...props} onClick={() => onSelect?.()}>
      {children}
    </button>
  ),
}));

vi.mock("@/stores/projectStore", () => ({
  useCurrentProjectId: () => "proj-001",
}));

vi.mock("@/lib/api/handoff", () => ({
  generateHandoff: vi.fn(),
}));

vi.mock("@/stores/uiStore", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { generateHandoff } from "@/lib/api/handoff";
import { toast } from "@/stores/uiStore";

describe("HandoffActions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    });
    vi.mocked(generateHandoff).mockResolvedValue(
      create(GenerateHandoffResponseSchema, {
        contextPack: "context pack",
        bootstrapPrompt: "<context>\ncontext pack\n</context>",
        cliCommand: "claude -p 'context pack'",
      }),
    );
  });

  it("calls GenerateHandoff with source props and copies the Claude command", async () => {
    render(
      <HandoffActions
        sourceType={HandoffSourceType.TASK}
        sourceId="TASK-001"
      />,
    );

    fireEvent.pointerDown(
      screen.getByRole("button", { name: "Handoff actions" }),
      { button: 0 },
    );
    fireEvent.click(await screen.findByText("Copy Claude command"));

    await waitFor(() => {
      expect(generateHandoff).toHaveBeenCalledWith(
        "proj-001",
        HandoffSourceType.TASK,
        "TASK-001",
        HandoffTarget.CLAUDE_CODE,
      );
    });
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        "claude -p 'context pack'",
      );
    });
    expect(toast.success).toHaveBeenCalledWith("Claude handoff command copied");
  });

  it("copies the Codex command for the codex action", async () => {
    vi.mocked(generateHandoff).mockResolvedValue(
      create(GenerateHandoffResponseSchema, {
        contextPack: "context pack",
        bootstrapPrompt: "<context>\ncontext pack\n</context>",
        cliCommand: "codex 'context pack'",
      }),
    );

    render(
      <HandoffActions
        sourceType={HandoffSourceType.RECOMMENDATION}
        sourceId="REC-001"
      />,
    );

    fireEvent.pointerDown(
      screen.getByRole("button", { name: "Handoff actions" }),
      { button: 0 },
    );
    fireEvent.click(await screen.findByText("Copy Codex command"));

    await waitFor(() => {
      expect(generateHandoff).toHaveBeenCalledWith(
        "proj-001",
        HandoffSourceType.RECOMMENDATION,
        "REC-001",
        HandoffTarget.CODEX,
      );
    });
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        "codex 'context pack'",
      );
    });
    expect(toast.success).toHaveBeenCalledWith("Codex handoff command copied");
  });

  it("shows an error toast when the API call fails", async () => {
    vi.mocked(generateHandoff).mockRejectedValue(new Error("handoff blew up"));

    render(
      <HandoffActions
        sourceType={HandoffSourceType.THREAD}
        sourceId="THR-001"
      />,
    );

    fireEvent.pointerDown(
      screen.getByRole("button", { name: "Handoff actions" }),
      { button: 0 },
    );
    fireEvent.click(await screen.findByText("Copy bootstrap prompt"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("handoff blew up");
    });
  });

  it("shows an error toast when clipboard write fails", async () => {
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockRejectedValue(new Error("clipboard denied")),
      },
    });

    render(
      <HandoffActions
        sourceType={HandoffSourceType.ATTENTION_ITEM}
        sourceId="proj-001:failed-TASK-001"
      />,
    );

    fireEvent.pointerDown(
      screen.getByRole("button", { name: "Handoff actions" }),
      { button: 0 },
    );
    fireEvent.click(await screen.findByText("Copy context pack"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("clipboard denied");
    });
  });
});
