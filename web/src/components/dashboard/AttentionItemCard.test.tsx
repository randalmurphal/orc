import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { AttentionItemCard } from "./AttentionItemCard";
import {
  AttentionAction,
  AttentionItemSchema,
  AttentionItemType,
} from "@/gen/orc/v1/attention_dashboard_pb";
import { HandoffSourceType } from "@/gen/orc/v1/handoff_pb";

vi.mock("@/components/handoff/HandoffActions", () => ({
  HandoffActions: ({
    sourceType,
    sourceId,
  }: {
    sourceType: number;
    sourceId: string;
  }) => (
    <div
      data-testid="handoff-actions"
      data-source-type={String(sourceType)}
      data-source-id={sourceId}
    />
  ),
}));

describe("AttentionItemCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders handoff actions with attention item source wiring", () => {
    render(
      <AttentionItemCard
        item={create(AttentionItemSchema, {
          id: "proj-001:failed-TASK-001",
          type: AttentionItemType.FAILED_TASK,
          taskId: "TASK-001",
          title: "TASK-001 failed",
          description: "Review failed.",
          projectId: "proj-001",
          availableActions: [AttentionAction.RETRY, AttentionAction.VIEW],
        })}
        projectName="Project One"
        onOpen={vi.fn()}
        onAction={vi.fn().mockResolvedValue(undefined)}
      />,
    );

    expect(screen.getByTestId("handoff-actions")).toHaveAttribute(
      "data-source-type",
      String(HandoffSourceType.ATTENTION_ITEM),
    );
    expect(screen.getByTestId("handoff-actions")).toHaveAttribute(
      "data-source-id",
      "proj-001:failed-TASK-001",
    );
  });
});
