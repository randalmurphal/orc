import { create } from 'zustand';
import { useShallow } from 'zustand/react/shallow';
import { subscribeWithSelector } from 'zustand/middleware';
import {
	type Workflow,
	type WorkflowWithDetails,
	type PhaseTemplate,
	type WorkflowRun,
	type WorkflowRunWithDetails,
	RunStatus,
	DefinitionSource,
} from '@/gen/orc/v1/workflow_pb';

interface WorkflowStore {
	// State
	workflows: Workflow[];
	phaseTemplates: PhaseTemplate[];
	workflowRuns: WorkflowRun[];
	workflowSources: Record<string, DefinitionSource>;
	phaseSources: Record<string, DefinitionSource>;
	selectedWorkflow: WorkflowWithDetails | null;
	selectedRun: WorkflowRunWithDetails | null;
	loading: boolean;
	error: string | null;
	refreshKey: number; // Incremented to trigger re-fetches

	// Derived state
	getBuiltinWorkflows: () => Workflow[];
	getCustomWorkflows: () => Workflow[];
	getBuiltinPhases: () => PhaseTemplate[];
	getCustomPhases: () => PhaseTemplate[];
	getRunningRuns: () => WorkflowRun[];

	// Actions
	setWorkflows: (workflows: Workflow[], sources?: Record<string, DefinitionSource>) => void;
	setPhaseTemplates: (templates: PhaseTemplate[], sources?: Record<string, DefinitionSource>) => void;
	setWorkflowRuns: (runs: WorkflowRun[]) => void;
	setSelectedWorkflow: (workflow: WorkflowWithDetails | null) => void;
	setSelectedRun: (run: WorkflowRunWithDetails | null) => void;
	addWorkflow: (workflow: Workflow) => void;
	updateWorkflow: (id: string, updates: Partial<Workflow>) => void;
	removeWorkflow: (id: string) => void;
	addPhaseTemplate: (template: PhaseTemplate) => void;
	updatePhaseTemplate: (id: string, updates: Partial<PhaseTemplate>) => void;
	removePhaseTemplate: (id: string) => void;
	refreshPhaseTemplates: () => void;
	addWorkflowRun: (run: WorkflowRun) => void;
	updateWorkflowRun: (id: string, updates: Partial<WorkflowRun>) => void;
	setLoading: (loading: boolean) => void;
	setError: (error: string | null) => void;
	reset: () => void;
}

const initialState = {
	workflows: [] as Workflow[],
	phaseTemplates: [] as PhaseTemplate[],
	workflowRuns: [] as WorkflowRun[],
	workflowSources: {} as Record<string, DefinitionSource>,
	phaseSources: {} as Record<string, DefinitionSource>,
	selectedWorkflow: null as WorkflowWithDetails | null,
	selectedRun: null as WorkflowRunWithDetails | null,
	loading: false,
	error: null as string | null,
	refreshKey: 0,
};

export const useWorkflowStore = create<WorkflowStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		// Derived state
		getBuiltinWorkflows: () => {
			return get().workflows.filter((wf) => wf.isBuiltin);
		},

		getCustomWorkflows: () => {
			return get().workflows.filter((wf) => !wf.isBuiltin);
		},

		getBuiltinPhases: () => {
			return get().phaseTemplates.filter((pt) => pt.isBuiltin);
		},

		getCustomPhases: () => {
			return get().phaseTemplates.filter((pt) => !pt.isBuiltin);
		},

		getRunningRuns: () => {
			return get().workflowRuns.filter((run) => run.status === RunStatus.RUNNING);
		},

		// Actions
		setWorkflows: (workflows, sources) => set({
			workflows,
			workflowSources: sources ?? {},
			error: null
		}),

		setPhaseTemplates: (phaseTemplates, sources) => set({
			phaseTemplates,
			phaseSources: sources ?? {},
			error: null
		}),

		setWorkflowRuns: (workflowRuns) => set({ workflowRuns, error: null }),

		setSelectedWorkflow: (selectedWorkflow) => set({ selectedWorkflow }),

		setSelectedRun: (selectedRun) => set({ selectedRun }),

		addWorkflow: (workflow) =>
			set((state) => {
				if (state.workflows.some((wf) => wf.id === workflow.id)) {
					return state;
				}
				return { workflows: [...state.workflows, workflow] };
			}),

		updateWorkflow: (id, updates) =>
			set((state) => ({
				workflows: state.workflows.map((wf) =>
					wf.id === id ? { ...wf, ...updates } : wf
				),
			})),

		removeWorkflow: (id) =>
			set((state) => ({
				workflows: state.workflows.filter((wf) => wf.id !== id),
			})),

		addPhaseTemplate: (template) =>
			set((state) => {
				if (state.phaseTemplates.some((pt) => pt.id === template.id)) {
					return state;
				}
				return { phaseTemplates: [...state.phaseTemplates, template] };
			}),

		updatePhaseTemplate: (id, updates) =>
			set((state) => ({
				phaseTemplates: state.phaseTemplates.map((pt) =>
					pt.id === id ? { ...pt, ...updates } : pt
				),
			})),

		removePhaseTemplate: (id) =>
			set((state) => ({
				phaseTemplates: state.phaseTemplates.filter((pt) => pt.id !== id),
			})),

		refreshPhaseTemplates: () =>
			set((state) => ({ refreshKey: state.refreshKey + 1 })),

		addWorkflowRun: (run) =>
			set((state) => {
				if (state.workflowRuns.some((r) => r.id === run.id)) {
					return state;
				}
				return { workflowRuns: [...state.workflowRuns, run] };
			}),

		updateWorkflowRun: (id, updates) =>
			set((state) => ({
				workflowRuns: state.workflowRuns.map((run) =>
					run.id === id ? { ...run, ...updates } : run
				),
			})),

		setLoading: (loading) => set({ loading }),

		setError: (error) => set({ error }),

		reset: () => set(initialState),
	}))
);

// Selector hooks
export const useWorkflows = () => useWorkflowStore((state) => state.workflows);
export const usePhaseTemplates = () => useWorkflowStore((state) => state.phaseTemplates);
export const useWorkflowRuns = () => useWorkflowStore((state) => state.workflowRuns);
export const useWorkflowSources = () => useWorkflowStore((state) => state.workflowSources);
export const usePhaseSources = () => useWorkflowStore((state) => state.phaseSources);
// useShallow prevents re-renders when filter results haven't changed —
// these methods return new arrays on every call, but useShallow compares elements.
export const useBuiltinWorkflows = () => useWorkflowStore(useShallow((state) => state.getBuiltinWorkflows()));
export const useCustomWorkflows = () => useWorkflowStore(useShallow((state) => state.getCustomWorkflows()));
export const useBuiltinPhases = () => useWorkflowStore(useShallow((state) => state.getBuiltinPhases()));
export const useCustomPhases = () => useWorkflowStore(useShallow((state) => state.getCustomPhases()));
export const useRunningRuns = () => useWorkflowStore(useShallow((state) => state.getRunningRuns()));
export const useSelectedWorkflow = () => useWorkflowStore((state) => state.selectedWorkflow);
export const useSelectedRun = () => useWorkflowStore((state) => state.selectedRun);
export const useRefreshKey = () => useWorkflowStore((state) => state.refreshKey);
