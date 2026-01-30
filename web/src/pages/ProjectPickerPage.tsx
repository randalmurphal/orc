/**
 * ProjectPickerPage - Shown when no project is selected
 *
 * This page is displayed when:
 * - First visit with no projects registered
 * - Current project was removed
 * - User explicitly cleared project selection
 */

import { useMemo, useState, useEffect, useRef, useCallback } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useProjectStore } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import './ProjectPickerPage.css';

export function ProjectPickerPage() {
	useDocumentTitle('Select Project');

	const projects = useProjectStore((state) => state.projects);
	const loading = useProjectStore((state) => state.loading);
	const error = useProjectStore((state) => state.error);
	const selectProject = useProjectStore((state) => state.selectProject);

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

	// Focus input on mount
	useEffect(() => {
		inputRef.current?.focus();
	}, []);

	// Reset selected index when search changes
	useEffect(() => {
		setSelectedIndex(0);
	}, [searchQuery]);

	const handleSelect = useCallback(
		(id: string) => {
			selectProject(id);
		},
		[selectProject]
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
			}
		},
		[filteredProjects, selectedIndex, handleSelect]
	);

	return (
		<div className="project-picker-page" onKeyDown={handleKeydown}>
			<div className="project-picker-page__container">
				{/* Header */}
				<div className="project-picker-page__header">
					<div className="project-picker-page__logo">
						<Icon name="layers" size={32} />
					</div>
					<h1 className="project-picker-page__title">Welcome to orc</h1>
					<p className="project-picker-page__subtitle">
						Select a project to get started
					</p>
				</div>

				{/* Content card */}
				<div className="project-picker-page__card">
					{/* Search */}
					<div className="project-picker-page__search">
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
					<div className="project-picker-page__list">
						{loading ? (
							<div className="project-picker-page__state">
								<div className="spinner" />
								<span>Loading projects...</span>
							</div>
						) : error ? (
							<div className="project-picker-page__state project-picker-page__state--error">
								<div className="error-icon">!</div>
								<span className="error-message">{error}</span>
							</div>
						) : filteredProjects.length === 0 ? (
							<div className="project-picker-page__state project-picker-page__state--empty">
								{searchQuery.trim() ? (
									<>
										<Icon name="search" size={32} />
										<p>No projects match "{searchQuery}"</p>
									</>
								) : (
									<>
										<Icon name="folder" size={32} />
										<p>No projects registered</p>
										<div className="project-picker-page__hint">
											<span>Run</span>
											<code>orc init</code>
											<span>in a project directory to get started</span>
										</div>
									</>
								)}
							</div>
						) : (
							filteredProjects.map((project, i) => (
								<button
									key={project.id}
									className={`project-picker-page__item ${i === selectedIndex ? 'selected' : ''}`}
									onClick={() => handleSelect(project.id)}
									onMouseEnter={() => setSelectedIndex(i)}
								>
									<div className="project-icon">
										<Icon name="folder" size={18} />
									</div>
									<div className="project-info">
										<span className="project-name">{project.name}</span>
										<span className="project-path">{project.path}</span>
									</div>
									<Icon name="chevron-right" size={16} className="project-arrow" />
								</button>
							))
						)}
					</div>
				</div>

				{/* Footer hints */}
				{filteredProjects.length > 0 && (
					<div className="project-picker-page__footer">
						<div className="footer-hint">
							<kbd>&uarr;</kbd><kbd>&darr;</kbd> navigate
						</div>
						<div className="footer-hint">
							<kbd>&crarr;</kbd> select
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
