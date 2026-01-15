import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { createPortal } from 'react-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import { useProjectStore, useCurrentProjectId } from '@/stores';
import './ProjectSwitcher.css';

interface ProjectSwitcherProps {
	open: boolean;
	onClose: () => void;
}

/**
 * Modal for switching between projects.
 *
 * Features:
 * - Search/filter projects
 * - Keyboard navigation (arrows, enter, escape)
 * - Shows current project with badge
 * - Loading/error states
 */
export function ProjectSwitcher({ open, onClose }: ProjectSwitcherProps) {
	const projects = useProjectStore((state) => state.projects);
	const currentProjectId = useCurrentProjectId();
	const selectProject = useProjectStore((state) => state.selectProject);
	const loading = useProjectStore((state) => state.loading);
	const error = useProjectStore((state) => state.error);

	const [searchQuery, setSearchQuery] = useState('');
	const [selectedIndex, setSelectedIndex] = useState(0);
	const inputRef = useRef<HTMLInputElement>(null);

	// Filter projects by search query
	const filteredProjects = useMemo(() => {
		if (!searchQuery.trim()) return projects;
		const query = searchQuery.toLowerCase();
		return projects.filter(
			(p) =>
				p.name.toLowerCase().includes(query) ||
				p.path.toLowerCase().includes(query)
		);
	}, [projects, searchQuery]);

	// Get current project
	const currentProject = useMemo(() => {
		return projects.find((p) => p.id === currentProjectId);
	}, [projects, currentProjectId]);

	// Reset state when modal opens
	useEffect(() => {
		if (open) {
			setSearchQuery('');
			setSelectedIndex(0);
			// Focus input after a short delay to ensure modal is rendered
			requestAnimationFrame(() => {
				inputRef.current?.focus();
			});
		}
	}, [open]);

	// Reset selected index when search changes
	useEffect(() => {
		setSelectedIndex(0);
	}, [searchQuery]);

	const handleSelect = useCallback(
		(id: string) => {
			selectProject(id);
			onClose();
		},
		[selectProject, onClose]
	);

	const handleKeydown = useCallback(
		(e: React.KeyboardEvent) => {
			switch (e.key) {
				case 'ArrowDown':
					e.preventDefault();
					setSelectedIndex((prev) => Math.min(prev + 1, filteredProjects.length - 1));
					break;
				case 'ArrowUp':
					e.preventDefault();
					setSelectedIndex((prev) => Math.max(prev - 1, 0));
					break;
				case 'Enter':
					e.preventDefault();
					if (filteredProjects[selectedIndex]) {
						handleSelect(filteredProjects[selectedIndex].id);
					}
					break;
				case 'Escape':
					e.preventDefault();
					onClose();
					break;
			}
		},
		[filteredProjects, selectedIndex, handleSelect, onClose]
	);

	const handleBackdropClick = useCallback(
		(e: React.MouseEvent) => {
			if (e.target === e.currentTarget) {
				onClose();
			}
		},
		[onClose]
	);

	if (!open) return null;

	const content = (
		<div
			className="switcher-backdrop"
			role="dialog"
			aria-modal="true"
			aria-label="Switch project"
			tabIndex={-1}
			onClick={handleBackdropClick}
			onKeyDown={handleKeydown}
		>
			<div className="switcher-content">
				{/* Header */}
				<div className="switcher-header">
					<h2>Switch Project</h2>
					<Button
						variant="ghost"
						size="sm"
						iconOnly
						className="close-btn"
						onClick={onClose}
						aria-label="Close"
						title="Close (Esc)"
					>
						<Icon name="close" size={16} />
					</Button>
				</div>

				{/* Current Project */}
				{currentProject && (
					<div className="current-project">
						<span className="current-label">Current</span>
						<div className="current-info">
							<span className="current-name">{currentProject.name}</span>
							<span className="current-path">{currentProject.path}</span>
						</div>
					</div>
				)}

				{/* Search Input */}
				<div className="switcher-search">
					<Icon name="search" size={16} className="search-icon" />
					<input
						ref={inputRef}
						type="text"
						value={searchQuery}
						onChange={(e) => setSearchQuery(e.target.value)}
						placeholder="Search projects..."
						className="search-input"
						aria-label="Search projects"
					/>
				</div>

				{/* Project List */}
				<div className="project-list">
					{loading ? (
						<div className="loading-state">
							<div className="spinner" />
							<span>Loading projects...</span>
						</div>
					) : error ? (
						<div className="error-state">
							<span className="error-icon">!</span>
							<span className="error-message">{error}</span>
						</div>
					) : filteredProjects.length === 0 ? (
						<div className="empty-state">
							{searchQuery.trim() ? (
								<p>No projects match "{searchQuery}"</p>
							) : (
								<>
									<p>No projects registered</p>
									<span className="empty-hint">Run `orc init` in a project directory</span>
								</>
							)}
						</div>
					) : (
						filteredProjects.map((project, i) => (
							<button
								key={project.id}
								className={`project-item ${i === selectedIndex ? 'selected' : ''} ${project.id === currentProjectId ? 'active' : ''}`}
								onClick={() => handleSelect(project.id)}
								onMouseEnter={() => setSelectedIndex(i)}
							>
								<div className="project-icon">
									{project.id === currentProjectId ? (
										<Icon name="check" size={14} />
									) : (
										<Icon name="folder" size={14} />
									)}
								</div>
								<div className="project-info">
									<span className="project-name">{project.name}</span>
									<span className="project-path">{project.path}</span>
								</div>
								{project.id === currentProjectId && (
									<span className="active-badge">Active</span>
								)}
							</button>
						))
					)}
				</div>

				{/* Footer */}
				<div className="switcher-footer">
					<div className="footer-hint">
						<kbd>↑</kbd><kbd>↓</kbd> navigate
					</div>
					<div className="footer-hint">
						<kbd>↵</kbd> select
					</div>
					<div className="footer-hint">
						<kbd>esc</kbd> close
					</div>
				</div>
			</div>
		</div>
	);

	return createPortal(content, document.body);
}
