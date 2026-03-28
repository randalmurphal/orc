import { create } from '@bufbuild/protobuf';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import { taskClient } from '@/lib/client';
import { useCurrentProject, useCurrentProjectId, useProjectStore, useTaskStore, useThreadStore } from '@/stores';
import { toast } from '@/stores/uiStore';
import {
	FinalizeTaskRequestSchema,
	ResumeTaskRequestSchema,
	type Task,
	TaskStatus,
	UpdateTaskRequestSchema,
} from '@/gen/orc/v1/task_pb';
import type { Thread } from '@/gen/orc/v1/thread_pb';
import './CommandPalette.css';

const NEW_TASK_EVENT = 'orc:new-task';
const PROJECT_SWITCHER_EVENT = 'orc:project-switcher';

type CommandKind =
	| 'navigation'
	| 'global'
	| 'thread'
	| 'show'
	| 'log'
	| 'resume'
	| 'approve'
	| 'finalize'
	| 'close'
	| 'copy';

interface CommandItem {
	id: string;
	kind: CommandKind;
	label: string;
	description: string;
	section: string;
	searchText: string;
	taskId?: string;
	threadId?: string;
	cliCommand?: string;
	path?: string;
}

interface CommandPaletteProps {
	open: boolean;
	onClose: () => void;
}

function normalize(text: string): string {
	return text.trim().toLowerCase();
}

function getProjectId(candidate: unknown): string | null {
	if (!candidate || typeof candidate !== 'object') {
		return null;
	}
	const projectId = Reflect.get(candidate, 'projectId');
	return typeof projectId === 'string' && projectId.length > 0 ? projectId : null;
}

function belongsToProject(projectId: string | null, candidate: unknown): boolean {
	if (!projectId) {
		return false;
	}
	const candidateProjectId = getProjectId(candidate);
	if (!candidateProjectId) {
		return true;
	}
	return candidateProjectId === projectId;
}

function canFinalizeTask(task: Task): boolean {
	return [
		TaskStatus.COMPLETED,
		TaskStatus.FINALIZING,
	].includes(task.status);
}

function canResumeTask(task: Task): boolean {
	return [TaskStatus.PAUSED, TaskStatus.FAILED, TaskStatus.BLOCKED].includes(task.status);
}

function canApproveTask(task: Task): boolean {
	return task.status === TaskStatus.BLOCKED;
}

function canCloseTask(task: Task): boolean {
	return task.status === TaskStatus.FAILED;
}

function assertTaskActionAllowed(task: Task, action: CommandKind): void {
	switch (action) {
		case 'resume':
			if (canResumeTask(task)) {
				return;
			}
			throw new Error(`Task ${task.id} cannot be resumed from its current status`);
		case 'approve':
			if (canApproveTask(task)) {
				return;
			}
			throw new Error(`Task ${task.id} is not blocked`);
		case 'finalize':
			if (canFinalizeTask(task)) {
				return;
			}
			throw new Error(`Task ${task.id} cannot be finalized from its current status`);
		case 'close':
			if (canCloseTask(task)) {
				return;
			}
			throw new Error(`Task ${task.id} is not failed`);
	}
}

function createTaskCommandItems(task: Task): CommandItem[] {
	const items: CommandItem[] = [
		{
			id: `show:${task.id}`,
			kind: 'show',
			label: `Show ${task.id}`,
			description: task.title,
			section: 'Tasks',
			searchText: normalize(`show ${task.id} ${task.title} task details`),
			taskId: task.id,
			cliCommand: `orc show ${task.id}`,
		},
		{
			id: `copy:show:${task.id}`,
			kind: 'copy',
			label: `Copy: orc show ${task.id}`,
			description: task.title,
			section: 'CLI',
			searchText: normalize(`copy orc show ${task.id} ${task.title}`),
			taskId: task.id,
			cliCommand: `orc show ${task.id}`,
		},
		{
			id: `log:${task.id}`,
			kind: 'log',
			label: `Log ${task.id}`,
			description: task.title,
			section: 'Tasks',
			searchText: normalize(`log ${task.id} ${task.title} transcript`),
			taskId: task.id,
			cliCommand: `orc log ${task.id}`,
		},
		{
			id: `copy:log:${task.id}`,
			kind: 'copy',
			label: `Copy: orc log ${task.id}`,
			description: task.title,
			section: 'CLI',
			searchText: normalize(`copy orc log ${task.id} ${task.title}`),
			taskId: task.id,
			cliCommand: `orc log ${task.id}`,
		},
	];

	if (canResumeTask(task)) {
		items.push(
			{
				id: `resume:${task.id}`,
				kind: 'resume',
				label: `Resume ${task.id}`,
				description: task.title,
				section: 'Tasks',
				searchText: normalize(`resume ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc resume ${task.id}`,
			},
			{
				id: `copy:resume:${task.id}`,
				kind: 'copy',
				label: `Copy: orc resume ${task.id}`,
				description: task.title,
				section: 'CLI',
				searchText: normalize(`copy orc resume ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc resume ${task.id}`,
			}
		);
	}

	if (canApproveTask(task)) {
		items.push(
			{
				id: `approve:${task.id}`,
				kind: 'approve',
				label: `Approve ${task.id}`,
				description: task.title,
				section: 'Tasks',
				searchText: normalize(`approve ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc approve ${task.id}`,
			},
			{
				id: `copy:approve:${task.id}`,
				kind: 'copy',
				label: `Copy: orc approve ${task.id}`,
				description: task.title,
				section: 'CLI',
				searchText: normalize(`copy orc approve ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc approve ${task.id}`,
			}
		);
	}

	if (canFinalizeTask(task)) {
		items.push(
			{
				id: `finalize:${task.id}`,
				kind: 'finalize',
				label: `Finalize ${task.id}`,
				description: task.title,
				section: 'Tasks',
				searchText: normalize(`finalize ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc finalize ${task.id}`,
			},
			{
				id: `copy:finalize:${task.id}`,
				kind: 'copy',
				label: `Copy: orc finalize ${task.id}`,
				description: task.title,
				section: 'CLI',
				searchText: normalize(`copy orc finalize ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc finalize ${task.id}`,
			}
		);
	}

	if (canCloseTask(task)) {
		items.push(
			{
				id: `close:${task.id}`,
				kind: 'close',
				label: `Close ${task.id}`,
				description: task.title,
				section: 'Tasks',
				searchText: normalize(`close ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc close ${task.id}`,
			},
			{
				id: `copy:close:${task.id}`,
				kind: 'copy',
				label: `Copy: orc close ${task.id}`,
				description: task.title,
				section: 'CLI',
				searchText: normalize(`copy orc close ${task.id} ${task.title}`),
				taskId: task.id,
				cliCommand: `orc close ${task.id}`,
			}
		);
	}

	return items;
}

function createThreadCommandItem(thread: Thread): CommandItem {
	return {
		id: `thread:${thread.id}`,
		kind: 'thread',
		label: `Open thread: ${thread.title}`,
		description: thread.taskId ? `Linked to ${thread.taskId}` : 'Discussion thread',
		section: 'Threads',
		searchText: normalize(`open thread ${thread.title} ${thread.taskId}`),
		threadId: thread.id,
	};
}

function isOpenThread(thread: Thread): boolean {
	return thread.status !== 'archived';
}

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
	const navigate = useNavigate();
	const currentProject = useCurrentProject();
	const projectId = useCurrentProjectId();
	const tasks = useTaskStore((state) => state.tasks);
	const threads = useThreadStore((state) => state.threads);
	const selectThread = useThreadStore((state) => state.selectThread);
	const inputRef = useRef<HTMLInputElement>(null);
	const listRef = useRef<HTMLDivElement>(null);
	const [query, setQuery] = useState('');
	const [selectedIndex, setSelectedIndex] = useState(0);
	const [isRunningAction, setIsRunningAction] = useState(false);

	const commandItems = useMemo(() => {
		if (!open) {
			return [];
		}

		const navigationItems: CommandItem[] = [
			{
				id: 'nav:home',
				kind: 'navigation',
				label: 'Go to Home',
				description: 'My work across projects',
				section: 'Navigation',
				searchText: normalize('go to home dashboard my work'),
				path: '/',
			},
			{
				id: 'nav:project',
				kind: 'navigation',
				label: 'Go to Project',
				description: 'Project home and activity',
				section: 'Navigation',
				searchText: normalize('go to project home'),
				path: '/project',
			},
			{
				id: 'nav:board',
				kind: 'navigation',
				label: 'Go to Board',
				description: 'Task board and execution state',
				section: 'Navigation',
				searchText: normalize('go to board tasks'),
				path: '/board',
			},
			{
				id: 'nav:recommendations',
				kind: 'navigation',
				label: 'Go to Inbox',
				description: 'Recommendations and follow-up',
				section: 'Navigation',
				searchText: normalize('go to inbox recommendations'),
				path: '/recommendations',
			},
			{
				id: 'nav:initiatives',
				kind: 'navigation',
				label: 'Go to Initiatives',
				description: 'Initiative overview',
				section: 'Navigation',
				searchText: normalize('go to initiatives'),
				path: '/initiatives',
			},
			{
				id: 'nav:timeline',
				kind: 'navigation',
				label: 'Go to Timeline',
				description: 'Recent activity feed',
				section: 'Navigation',
				searchText: normalize('go to timeline'),
				path: '/timeline',
			},
			{
				id: 'nav:stats',
				kind: 'navigation',
				label: 'Go to Stats',
				description: 'Project statistics',
				section: 'Navigation',
				searchText: normalize('go to stats analytics'),
				path: '/stats',
			},
			{
				id: 'nav:workflows',
				kind: 'navigation',
				label: 'Go to Workflows',
				description: 'Workflow definitions',
				section: 'Navigation',
				searchText: normalize('go to workflows'),
				path: '/workflows',
			},
			{
				id: 'nav:settings',
				kind: 'navigation',
				label: 'Go to Settings',
				description: 'Configuration and environment',
				section: 'Navigation',
				searchText: normalize('go to settings config'),
				path: '/settings',
			},
		];

		const globalItems: CommandItem[] = [
			{
				id: 'global:new-task',
				kind: 'global',
				label: 'New Task',
				description: 'Open the workflow-first task modal',
				section: 'Actions',
				searchText: normalize('new task create task'),
			},
			{
				id: 'global:switch-project',
				kind: 'global',
				label: 'Switch Project',
				description: 'Open the project switcher',
				section: 'Actions',
				searchText: normalize('switch project change project'),
			},
		];

		const threadItems = threads
			.filter((thread) => belongsToProject(projectId, thread))
			.filter(isOpenThread)
			.map(createThreadCommandItem);
		const taskItems = tasks
			.filter((task) => belongsToProject(projectId, task))
			.flatMap(createTaskCommandItems);

		return [...globalItems, ...navigationItems, ...threadItems, ...taskItems];
	}, [open, projectId, tasks, threads]);

	const filteredItems = useMemo(() => {
		const normalizedQuery = normalize(query);
		if (!normalizedQuery) {
			return commandItems;
		}
		return commandItems.filter((item) => item.searchText.includes(normalizedQuery));
	}, [commandItems, query]);

	useEffect(() => {
		if (!open) {
			return;
		}
		setQuery('');
		setSelectedIndex(0);
		requestAnimationFrame(() => {
			inputRef.current?.focus();
		});
	}, [open]);

	useEffect(() => {
		setSelectedIndex(0);
	}, [query, projectId]);

	useEffect(() => {
		const selectedItem = listRef.current?.querySelector<HTMLElement>('.command-palette-item.selected');
		selectedItem?.scrollIntoView({ block: 'nearest' });
	}, [selectedIndex]);

	useEffect(() => {
		if (selectedIndex < filteredItems.length) {
			return;
		}
		setSelectedIndex(Math.max(filteredItems.length - 1, 0));
	}, [filteredItems.length, selectedIndex]);

	const requireProjectId = useCallback((): string => {
		const currentProjectId = useProjectStore.getState().currentProjectId;
		if (!currentProjectId) {
			throw new Error('No project selected');
		}
		return currentProjectId;
	}, []);

	const requireCurrentTask = useCallback((taskId: string) => {
		const currentProjectId = useProjectStore.getState().currentProjectId;
		const task = useTaskStore.getState().tasks.find((candidate) => candidate.id === taskId);
		if (!task || !belongsToProject(currentProjectId, task)) {
			throw new Error(`Task ${taskId} is not available in the current project`);
		}
		return task;
	}, []);

	const requireCurrentThread = useCallback((threadId: string) => {
		const currentProjectId = useProjectStore.getState().currentProjectId;
		const thread = useThreadStore.getState().threads.find((candidate) => candidate.id === threadId);
		if (!thread || !belongsToProject(currentProjectId, thread) || !isOpenThread(thread)) {
			throw new Error('Thread is not available in the current project');
		}
		return thread;
	}, []);

	const executeItem = useCallback(async (item: CommandItem) => {
		switch (item.kind) {
			case 'navigation':
				navigate(item.path ?? '/');
				onClose();
				return;
			case 'global':
				window.dispatchEvent(
					new CustomEvent(item.id === 'global:new-task' ? NEW_TASK_EVENT : PROJECT_SWITCHER_EVENT)
				);
				onClose();
				return;
			case 'thread': {
				const thread = requireCurrentThread(item.threadId ?? '');
				selectThread(thread.id);
				onClose();
				return;
			}
			case 'show': {
				const task = requireCurrentTask(item.taskId ?? '');
				navigate(`/tasks/${task.id}`);
				onClose();
				return;
			}
			case 'log': {
				const task = requireCurrentTask(item.taskId ?? '');
				navigate(`/tasks/${task.id}?tab=transcript`);
				onClose();
				return;
			}
			case 'copy': {
				requireCurrentTask(item.taskId ?? '');
				if (!item.cliCommand) {
					throw new Error('Missing CLI command');
				}
				await navigator.clipboard.writeText(item.cliCommand);
				toast.success(`Copied ${item.cliCommand}`);
				onClose();
				return;
			}
			case 'resume': {
				const activeProjectId = requireProjectId();
				const task = requireCurrentTask(item.taskId ?? '');
				assertTaskActionAllowed(task, 'resume');
				const result = await taskClient.resumeTask(
					create(ResumeTaskRequestSchema, { projectId: activeProjectId, taskId: task.id })
				);
				if (!result.task) {
					throw new Error('Resume response did not include an updated task');
				}
				useTaskStore.getState().updateTask(task.id, result.task);
				toast.success(`Resumed ${task.id}`);
				onClose();
				return;
			}
			case 'approve': {
				const activeProjectId = requireProjectId();
				const task = requireCurrentTask(item.taskId ?? '');
				assertTaskActionAllowed(task, 'approve');
				const result = await taskClient.updateTask(
					create(UpdateTaskRequestSchema, {
						projectId: activeProjectId,
						taskId: task.id,
						status: TaskStatus.PLANNED,
					})
				);
				if (!result.task) {
					throw new Error('Approve response did not include an updated task');
				}
				useTaskStore.getState().updateTask(task.id, result.task);
				toast.success(`Approved ${task.id}`);
				onClose();
				return;
			}
			case 'finalize': {
				const activeProjectId = requireProjectId();
				const task = requireCurrentTask(item.taskId ?? '');
				assertTaskActionAllowed(task, 'finalize');
				const result = await taskClient.finalizeTask(
					create(FinalizeTaskRequestSchema, { projectId: activeProjectId, taskId: task.id })
				);
				if (!result.task) {
					throw new Error('Finalize response did not include an updated task');
				}
				useTaskStore.getState().updateTask(task.id, result.task);
				toast.success(`Finalizing ${task.id}`);
				onClose();
				return;
			}
			case 'close': {
				const activeProjectId = requireProjectId();
				const task = requireCurrentTask(item.taskId ?? '');
				assertTaskActionAllowed(task, 'close');
				const result = await taskClient.updateTask(
					create(UpdateTaskRequestSchema, {
						projectId: activeProjectId,
						taskId: task.id,
						status: TaskStatus.CLOSED,
					})
				);
				if (!result.task) {
					throw new Error('Close response did not include an updated task');
				}
				useTaskStore.getState().updateTask(task.id, result.task);
				toast.success(`Closed ${task.id}`);
				onClose();
			}
		}
	}, [navigate, onClose, requireCurrentTask, requireCurrentThread, requireProjectId, selectThread]);

	const handleSelect = useCallback(async (item: CommandItem | undefined) => {
		if (!item || isRunningAction) {
			return;
		}
		setIsRunningAction(true);
		try {
			await executeItem(item);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Command failed');
		} finally {
			setIsRunningAction(false);
		}
	}, [executeItem, isRunningAction]);

	const handleKeyDownCapture = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				event.stopPropagation();
				setSelectedIndex((current) => {
					if (filteredItems.length === 0) {
						return 0;
					}
					return Math.min(current + 1, filteredItems.length - 1);
				});
				break;
			case 'ArrowUp':
				event.preventDefault();
				event.stopPropagation();
				setSelectedIndex((current) => Math.max(current - 1, 0));
				break;
			case 'Enter':
				event.preventDefault();
				event.stopPropagation();
				void handleSelect(filteredItems[selectedIndex]);
				break;
			case 'Escape':
				event.preventDefault();
				event.stopPropagation();
				onClose();
				break;
		}
	}, [filteredItems, handleSelect, onClose, selectedIndex]);

	const handleBackdropClick = useCallback((event: React.MouseEvent<HTMLDivElement>) => {
		if (event.target === event.currentTarget) {
			onClose();
		}
	}, [onClose]);

	if (!open) {
		return null;
	}

	const selectedItemId = filteredItems[selectedIndex]?.id;
	const projectLabel = currentProject?.name ?? 'No project selected';

	return createPortal(
		<div
			className="command-palette-backdrop"
			role="dialog"
			aria-modal="true"
			aria-label="Command palette"
			tabIndex={-1}
			onClick={handleBackdropClick}
			onKeyDownCapture={handleKeyDownCapture}
		>
			<div className="command-palette">
				<div className="command-palette-header">
					<div>
						<p className="command-palette-eyebrow">Operator</p>
						<h2>Command Palette</h2>
					</div>
					<Button
						variant="ghost"
						size="sm"
						iconOnly
						className="close-btn"
						onClick={onClose}
						aria-label="Close command palette"
						title="Close (Esc)"
					>
						<Icon name="close" size={16} />
					</Button>
				</div>

				<div className="command-palette-project">
					<span className="command-palette-project-label">Project</span>
					<span className="command-palette-project-value">{projectLabel}</span>
				</div>

				<div className="command-palette-search">
					<Icon name="search" size={16} className="search-icon" />
					<input
						ref={inputRef}
						type="text"
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="Search commands, task IDs, threads..."
						aria-label="Search commands"
					/>
				</div>

				<div
					ref={listRef}
					className="command-palette-list"
					role="listbox"
					aria-activedescendant={selectedItemId}
				>
					{filteredItems.length === 0 ? (
						<div className="command-palette-empty">
							<p>No commands match "{query}"</p>
							<span>Try a task ID, route, or action name.</span>
						</div>
					) : (
						filteredItems.map((item, index) => (
							<button
								key={item.id}
								id={item.id}
								type="button"
								role="option"
								aria-selected={index === selectedIndex}
								className={`command-palette-item ${index === selectedIndex ? 'selected' : ''}`}
								onClick={() => void handleSelect(item)}
								onMouseEnter={() => setSelectedIndex(index)}
							>
								<div className="command-palette-item-main">
									<span className="command-palette-item-label">{item.label}</span>
									<span className="command-palette-item-description">{item.description}</span>
								</div>
								<span className="command-palette-item-section">{item.section}</span>
							</button>
						))
					)}
				</div>

				<div className="command-palette-footer">
					<div className="footer-hint">
						<kbd>↑</kbd>
						<kbd>↓</kbd>
						<span>navigate</span>
					</div>
					<div className="footer-hint">
						<kbd>↵</kbd>
						<span>run</span>
					</div>
					<div className="footer-hint">
						<kbd>esc</kbd>
						<span>close</span>
					</div>
				</div>
			</div>
		</div>,
		document.body
	);
}
