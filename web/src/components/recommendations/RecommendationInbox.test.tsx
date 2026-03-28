import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { RecommendationInbox } from "./RecommendationInbox";
import {
  AcceptRecommendationResponseSchema,
  DiscussRecommendationResponseSchema,
  ListRecommendationHistoryResponseSchema,
  RecommendationKind,
  RecommendationHistoryEntrySchema,
  RecommendationSchema,
  RecommendationStatus,
  RejectRecommendationResponseSchema,
  ListRecommendationsResponseSchema,
  type RecommendationHistoryEntry,
  type Recommendation,
} from "@/gen/orc/v1/recommendation_pb";
import { HandoffSourceType } from "@/gen/orc/v1/handoff_pb";

let currentProjectId = "proj-001";

vi.mock("@/stores/projectStore", () => ({
  useCurrentProjectId: () => currentProjectId,
}));

vi.mock("@/lib/api/recommendation", () => ({
  listRecommendations: vi.fn(),
  listRecommendationHistory: vi.fn(),
  acceptRecommendation: vi.fn(),
  rejectRecommendation: vi.fn(),
  discussRecommendation: vi.fn(),
}));

vi.mock("@/components/handoff/HandoffActions", () => ({
  HandoffActions: ({
    sourceType,
    sourceId,
  }: {
    sourceType: number;
    sourceId: string;
  }) => (
    <div
      data-testid={`handoff-actions-${sourceId}`}
      data-source-type={String(sourceType)}
      data-source-id={sourceId}
    />
  ),
}));

vi.mock("@/stores/uiStore", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import {
  acceptRecommendation,
  discussRecommendation,
  listRecommendationHistory,
  listRecommendations,
  rejectRecommendation,
} from "@/lib/api/recommendation";
import { emitRecommendationSignal } from "@/lib/events/recommendationSignals";

describe("RecommendationInbox", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    currentProjectId = "proj-001";
  });

  it("renders loading and then the empty state", async () => {
    vi.mocked(listRecommendations).mockResolvedValue(makeListResponse([]));

    render(<RecommendationInbox />);

    expect(screen.getByText("Loading recommendations...")).toBeInTheDocument();
    await screen.findByText("No recommendations yet.");
  });

  it("shows an error state and retries loading", async () => {
    vi.mocked(listRecommendations)
      .mockRejectedValueOnce(new Error("load failed"))
      .mockResolvedValueOnce(makeListResponse([makeRecommendation()]));

    render(<RecommendationInbox />);

    await screen.findByText("load failed");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    await screen.findByText("Clean up duplicate polling");
    expect(listRecommendations).toHaveBeenCalledTimes(2);
  });

  it("renders recommendations and allows follow-up decisions from discussed state", async () => {
    vi.mocked(listRecommendations).mockResolvedValue(
      makeListResponse([
        makeRecommendation(),
        makeRecommendation({
          id: "REC-002",
          status: RecommendationStatus.DISCUSSED,
          title: "Discussed follow-up",
          dedupeKey: "cleanup:task-001:discussed",
        }),
      ]),
    );

    render(<RecommendationInbox />);

    await screen.findByText("Recommendation Inbox");
    expect(
      screen.getByText("1 pending recommendations need a human decision."),
    ).toBeInTheDocument();

    const discussedCard = screen
      .getByText("Discussed follow-up")
      .closest(".recommendation-card");
    expect(discussedCard).not.toBeNull();
    const acceptButton = withinCard(discussedCard!, "Accept");
    const rejectButton = withinCard(discussedCard!, "Reject");
    const discussButton = withinCard(discussedCard!, "Discuss");

    expect(acceptButton).toBeEnabled();
    expect(rejectButton).toBeEnabled();
    expect(discussButton).toBeDisabled();
  });

  it("discusses a recommendation and refreshes the list", async () => {
    vi.mocked(listRecommendations)
      .mockResolvedValueOnce(makeListResponse([makeRecommendation()]))
      .mockResolvedValueOnce(
        makeListResponse([
          makeRecommendation({
            status: RecommendationStatus.DISCUSSED,
            decisionReason: "Needs a narrower plan.",
          }),
        ]),
      );
    vi.mocked(discussRecommendation).mockResolvedValue(
      create(DiscussRecommendationResponseSchema, {
        recommendation: makeRecommendation({
          status: RecommendationStatus.DISCUSSED,
          decisionReason: "Needs a narrower plan.",
        }),
        contextPack: "Recommendation REC-001\nKind: cleanup",
      }),
    );

    render(<RecommendationInbox />);

    await screen.findByText("Clean up duplicate polling");
    fireEvent.change(screen.getByLabelText("Decision note"), {
      target: { value: "Needs a narrower plan." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Discuss" }));

    await screen.findByText("Discussed");
    expect(discussRecommendation).toHaveBeenCalledWith(
      "proj-001",
      "REC-001",
      "operator",
      "Needs a narrower plan.",
    );
  });

  it("renders handoff actions for each recommendation with recommendation source wiring", async () => {
    vi.mocked(listRecommendations).mockResolvedValue(
      makeListResponse([makeRecommendation()]),
    );

    render(<RecommendationInbox />);

    await screen.findByText("Clean up duplicate polling");
    expect(screen.getByTestId("handoff-actions-REC-001")).toHaveAttribute(
      "data-source-type",
      String(HandoffSourceType.RECOMMENDATION),
    );
    expect(screen.getByTestId("handoff-actions-REC-001")).toHaveAttribute(
      "data-source-id",
      "REC-001",
    );
  });

  it("accepts and rejects recommendations through the API, preserves decision notes, and shows promoted artifacts", async () => {
    vi.mocked(listRecommendations)
      .mockResolvedValueOnce(
        makeListResponse([
          makeRecommendation(),
          makeRecommendation({
            id: "REC-002",
            title: "Reject me",
            dedupeKey: "cleanup:task-001:reject-me",
          }),
        ]),
      )
      .mockResolvedValueOnce(
        makeListResponse([
          makeRecommendation({
            status: RecommendationStatus.ACCEPTED,
            decisionReason: "Looks worth shipping.",
            decidedBy: "operator",
            promotedToType: "task",
            promotedToId: "TASK-099",
          }),
          makeRecommendation({
            id: "REC-002",
            title: "Reject me",
            dedupeKey: "cleanup:task-001:reject-me",
          }),
        ]),
      )
      .mockResolvedValueOnce(
        makeListResponse([
          makeRecommendation({
            status: RecommendationStatus.ACCEPTED,
            decisionReason: "Looks worth shipping.",
            decidedBy: "operator",
            promotedToType: "task",
            promotedToId: "TASK-099",
          }),
          makeRecommendation({
            id: "REC-002",
            title: "Reject me",
            status: RecommendationStatus.REJECTED,
            decisionReason: "Not worth the churn.",
            dedupeKey: "cleanup:task-001:reject-me",
          }),
        ]),
      );
    vi.mocked(acceptRecommendation).mockResolvedValue(
      create(AcceptRecommendationResponseSchema, {
        recommendation: makeRecommendation({
          status: RecommendationStatus.ACCEPTED,
          decisionReason: "Looks worth shipping.",
          decidedBy: "operator",
          promotedToType: "task",
          promotedToId: "TASK-099",
        }),
      }),
    );
    vi.mocked(rejectRecommendation).mockResolvedValue(
      create(RejectRecommendationResponseSchema, {
        recommendation: makeRecommendation({
          id: "REC-002",
          title: "Reject me",
          status: RecommendationStatus.REJECTED,
          decisionReason: "Not worth the churn.",
          dedupeKey: "cleanup:task-001:reject-me",
        }),
      }),
    );

    render(<RecommendationInbox />);

    await screen.findByText("Clean up duplicate polling");
    fireEvent.change(screen.getAllByLabelText("Decision note")[0], {
      target: { value: "Looks worth shipping." },
    });
    fireEvent.click(screen.getAllByRole("button", { name: "Accept" })[0]);

    await waitFor(() => {
      expect(acceptRecommendation).toHaveBeenCalledWith(
        "proj-001",
        "REC-001",
        "operator",
        "Looks worth shipping.",
      );
    });
    await screen.findByText("Task TASK-099");
    expect(screen.getByText("Looks worth shipping.")).toBeInTheDocument();

    fireEvent.change(screen.getAllByLabelText("Decision note")[1], {
      target: { value: "Not worth the churn." },
    });
    fireEvent.click(screen.getAllByRole("button", { name: "Reject" })[1]);

    await waitFor(() => {
      expect(rejectRecommendation).toHaveBeenCalledWith(
        "proj-001",
        "REC-002",
        "operator",
        "Not worth the churn.",
      );
    });
    expect(listRecommendations).toHaveBeenCalledTimes(3);
  });

  it("loads recommendation history only when requested and renders the audit trail", async () => {
    vi.mocked(listRecommendations).mockResolvedValue(
      makeListResponse([
        makeRecommendation({
          status: RecommendationStatus.ACCEPTED,
          decisionReason: "Looks worth shipping.",
          decidedBy: "operator",
          promotedToType: "task",
          promotedToId: "TASK-099",
        }),
      ]),
    );
    vi.mocked(listRecommendationHistory).mockResolvedValue(
      makeHistoryResponse([
        makeHistoryEntry({
          id: 2n,
          fromStatus: RecommendationStatus.PENDING,
          toStatus: RecommendationStatus.ACCEPTED,
          decidedBy: "operator",
          decisionReason: "Looks worth shipping.",
        }),
        makeHistoryEntry({
          id: 1n,
          fromStatus: RecommendationStatus.UNSPECIFIED,
          toStatus: RecommendationStatus.PENDING,
        }),
      ]),
    );

    render(<RecommendationInbox />);

    await screen.findByText("Clean up duplicate polling");
    expect(listRecommendationHistory).not.toHaveBeenCalled();

    fireEvent.click(screen.getAllByRole("button", { name: "Show history" })[0]);

    await screen.findByText(/Accepted from pending by operator/);
    expect(screen.getByText("Pending")).toBeInTheDocument();
    expect(listRecommendationHistory).toHaveBeenCalledWith(
      "proj-001",
      "REC-001",
    );

    fireEvent.click(screen.getByRole("button", { name: "Hide history" }));
    expect(
      screen.queryByText(/Accepted from pending by operator/),
    ).not.toBeInTheDocument();
  });

  it("invalidates cached history after a decision so reopened history refetches fresh entries", async () => {
    vi.mocked(listRecommendations)
      .mockResolvedValueOnce(makeListResponse([makeRecommendation()]))
      .mockResolvedValueOnce(
        makeListResponse([
          makeRecommendation({
            status: RecommendationStatus.ACCEPTED,
            decisionReason: "Looks worth shipping.",
            decidedBy: "operator",
            promotedToType: "task",
            promotedToId: "TASK-099",
          }),
        ]),
      );
    vi.mocked(listRecommendationHistory)
      .mockResolvedValueOnce(
        makeHistoryResponse([
          makeHistoryEntry({
            id: 1n,
            fromStatus: RecommendationStatus.UNSPECIFIED,
            toStatus: RecommendationStatus.PENDING,
          }),
        ]),
      )
      .mockResolvedValueOnce(
        makeHistoryResponse([
          makeHistoryEntry({
            id: 2n,
            fromStatus: RecommendationStatus.PENDING,
            toStatus: RecommendationStatus.ACCEPTED,
            decidedBy: "operator",
            decisionReason: "Looks worth shipping.",
          }),
          makeHistoryEntry({
            id: 1n,
            fromStatus: RecommendationStatus.UNSPECIFIED,
            toStatus: RecommendationStatus.PENDING,
          }),
        ]),
      );
    vi.mocked(acceptRecommendation).mockResolvedValue(
      create(AcceptRecommendationResponseSchema, {
        recommendation: makeRecommendation({
          status: RecommendationStatus.ACCEPTED,
          decisionReason: "Looks worth shipping.",
          decidedBy: "operator",
          promotedToType: "task",
          promotedToId: "TASK-099",
        }),
      }),
    );

    render(<RecommendationInbox />);

    await screen.findByText("Clean up duplicate polling");
    fireEvent.click(screen.getByRole("button", { name: "Show history" }));
    await screen.findByText("Pending");
    expect(listRecommendationHistory).toHaveBeenCalledTimes(1);

    fireEvent.change(screen.getByLabelText("Decision note"), {
      target: { value: "Looks worth shipping." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Accept" }));

    await screen.findByText("Task TASK-099");
    expect(
      screen.queryByText(/Accepted from pending by operator/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Show history" }));
    await screen.findByText(/Accepted from pending by operator/);
    expect(listRecommendationHistory).toHaveBeenCalledTimes(2);
  });

  it("refreshes when an external recommendation event arrives for the current project", async () => {
    vi.mocked(listRecommendations)
      .mockResolvedValueOnce(makeListResponse([makeRecommendation()]))
      .mockResolvedValueOnce(
        makeListResponse([
          makeRecommendation(),
          makeRecommendation({
            id: "REC-002",
            title: "New external recommendation",
            dedupeKey: "cleanup:task-001:new-external",
          }),
        ]),
      );

    render(<RecommendationInbox />);

    await screen.findByText("Clean up duplicate polling");
    await act(async () => {
      emitRecommendationSignal({
        projectId: "proj-001",
        recommendationId: "REC-002",
        type: "created",
      });
    });

    await screen.findByText("New external recommendation");
    expect(listRecommendations).toHaveBeenCalledTimes(2);
  });

  it("does not leak decision notes across project switches when recommendation ids overlap", async () => {
    vi.mocked(listRecommendations).mockImplementation(
      async (projectId: string) => {
        if (projectId === "proj-002") {
          return makeListResponse([
            makeRecommendation({
              title: "Project B recommendation",
              summary: "Different project, same local recommendation id.",
            }),
          ]);
        }
        return makeListResponse([
          makeRecommendation({ title: "Project A recommendation" }),
        ]);
      },
    );

    const view = render(<RecommendationInbox />);

    await screen.findByText("Project A recommendation");
    fireEvent.change(screen.getByLabelText("Decision note"), {
      target: { value: "Project A note" },
    });
    expect(screen.getByDisplayValue("Project A note")).toBeInTheDocument();

    currentProjectId = "proj-002";
    view.rerender(<RecommendationInbox />);

    await screen.findByText("Project B recommendation");
    expect(
      screen.queryByDisplayValue("Project A note"),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Decision note")).toHaveValue("");
  });

  it("ignores stale recommendation loads after switching projects", async () => {
    const staleLoad = createDeferred<ReturnType<typeof makeListResponse>>();
    vi.mocked(listRecommendations).mockImplementation((projectId: string) => {
      if (projectId === "proj-002") {
        return Promise.resolve(
          makeListResponse([
            makeRecommendation({
              title: "Project B recommendation",
              summary: "This belongs to the active project.",
            }),
          ]),
        );
      }
      return staleLoad.promise as never;
    });

    const view = render(<RecommendationInbox />);

    currentProjectId = "proj-002";
    view.rerender(<RecommendationInbox />);

    await screen.findByText("Project B recommendation");
    await act(async () => {
      staleLoad.resolve(
        makeListResponse([
          makeRecommendation({
            title: "Project A recommendation",
            summary: "This response arrived late and must be ignored.",
          }),
        ]),
      );
      await Promise.resolve();
    });

    expect(
      screen.queryByText("Project A recommendation"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Project B recommendation")).toBeInTheDocument();
  });

  it("does not trigger an old-project reload after a decision completes on a different active project", async () => {
    const acceptDeferred =
      createDeferred<
        ReturnType<typeof create<typeof AcceptRecommendationResponseSchema>>
      >();
    vi.mocked(listRecommendations).mockImplementation(
      async (projectId: string) => {
        if (projectId === "proj-002") {
          return makeListResponse([
            makeRecommendation({
              title: "Project B recommendation",
              summary: "Active project after the switch.",
            }),
          ]);
        }
        return makeListResponse([
          makeRecommendation({ title: "Project A recommendation" }),
        ]);
      },
    );
    vi.mocked(acceptRecommendation).mockReturnValue(
      acceptDeferred.promise as never,
    );

    const view = render(<RecommendationInbox />);

    await screen.findByText("Project A recommendation");
    fireEvent.click(screen.getByRole("button", { name: "Accept" }));

    currentProjectId = "proj-002";
    view.rerender(<RecommendationInbox />);
    await screen.findByText("Project B recommendation");
    const loadCallsBeforeResolve =
      vi.mocked(listRecommendations).mock.calls.length;

    await act(async () => {
      acceptDeferred.resolve(
        create(AcceptRecommendationResponseSchema, {
          recommendation: makeRecommendation({
            status: RecommendationStatus.ACCEPTED,
          }),
        }),
      );
      await Promise.resolve();
    });

    expect(screen.getByText("Project B recommendation")).toBeInTheDocument();
    expect(
      screen.queryByText("Project A recommendation"),
    ).not.toBeInTheDocument();
    expect(vi.mocked(listRecommendations).mock.calls).toHaveLength(
      loadCallsBeforeResolve,
    );
  });
});

function makeListResponse(recommendations: Recommendation[]) {
  return create(ListRecommendationsResponseSchema, { recommendations });
}

function makeHistoryResponse(history: RecommendationHistoryEntry[]) {
  return create(ListRecommendationHistoryResponseSchema, { history });
}

function makeRecommendation(
  overrides: Record<string, unknown> = {},
): Recommendation {
  return create(RecommendationSchema, {
    id: "REC-001",
    kind: RecommendationKind.CLEANUP,
    status: RecommendationStatus.PENDING,
    title: "Clean up duplicate polling",
    summary: "Two polling loops are doing the same work.",
    proposedAction: "Remove the legacy loop after validating the new path.",
    evidence: "Both loops hit the same endpoint every 5 seconds.",
    sourceTaskId: "TASK-001",
    sourceRunId: "RUN-001",
    sourceThreadId: "THR-001",
    dedupeKey: "cleanup:task-001:duplicate-polling",
    ...overrides,
  });
}

function makeHistoryEntry(
  overrides: Record<string, unknown> = {},
): RecommendationHistoryEntry {
  return create(RecommendationHistoryEntrySchema, {
    id: 1n,
    recommendationId: "REC-001",
    fromStatus: RecommendationStatus.UNSPECIFIED,
    toStatus: RecommendationStatus.PENDING,
    decisionReason: "",
    decidedBy: "",
    ...overrides,
  });
}

function withinCard(card: Element, label: string): HTMLButtonElement {
  const button = Array.from(card.querySelectorAll("button")).find(
    (candidate) => candidate.textContent?.trim() === label,
  );
  if (!(button instanceof HTMLButtonElement)) {
    throw new Error(`button ${label} not found`);
  }
  return button;
}

function createDeferred<T>() {
  let resolvePromise: ((value: T | PromiseLike<T>) => void) | undefined;
  let rejectPromise: ((reason?: unknown) => void) | undefined;
  const promise = new Promise<T>((resolve, reject) => {
    resolvePromise = resolve;
    rejectPromise = reject;
  });
  return {
    promise,
    resolve(value: T) {
      if (resolvePromise === undefined) {
        throw new Error("Deferred promise resolved before initialization");
      }
      resolvePromise(value);
    },
    reject(reason?: unknown) {
      if (rejectPromise === undefined) {
        throw new Error("Deferred promise rejected before initialization");
      }
      rejectPromise(reason);
    },
  };
}
