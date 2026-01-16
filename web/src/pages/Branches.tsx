/**
 * Branches page (/branches)
 *
 * Displays tracked orc-managed branches (initiative, staging, task).
 * Supports filtering by type and status, and provides cleanup actions.
 */

import { useState, useEffect, useCallback, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import { listBranches, updateBranchStatus, deleteBranch } from '@/lib/api';
import { toast } from '@/stores/uiStore';
import type { Branch, BranchType, BranchStatus } from '@/lib/types';
import { BRANCH_STATUS_CONFIG, BRANCH_TYPE_CONFIG } from '@/lib/types';
import './Branches.css';

// Filter options
const BRANCH_TYPES: { value: BranchType | ''; label: string }[] = [
	{ value: '', label: 'All Types' },
	{ value: 'initiative', label: 'Initiative' },
	{ value: 'staging', label: 'Staging' },
	{ value: 'task', label: 'Task' },
];

const BRANCH_STATUSES: { value: BranchStatus | ''; label: string }[] = [
	{ value: '', label: 'All Statuses' },
	{ value: 'active', label: 'Active' },
	{ value: 'merged', label: 'Merged' },
	{ value: 'stale', label: 'Stale' },
	{ value: 'orphaned', label: 'Orphaned' },
];

// Format relative time
function formatTimeAgo(dateStr: string): string {
	const date = new Date(dateStr);
	const now = new Date();
	const diff = now.getTime() - date.getTime();
	const seconds = Math.floor(diff / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);
	const days = Math.floor(hours / 24);

	if (days > 0) return `${days}d ago`;
	if (hours > 0) return `${hours}h ago`;
	if (minutes > 0) return `${minutes}m ago`;
	return 'just now';
}

export function Branches() {
	const [branches, setBranches] = useState<Branch[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [typeFilter, setTypeFilter] = useState<BranchType | ''>('');
	const [statusFilter, setStatusFilter] = useState<BranchStatus | ''>('');
	const [actionLoading, setActionLoading] = useState<string | null>(null);

	const loadBranches = useCallback(async () => {
		try {
			setLoading(true);
			const data = await listBranches({
				type: typeFilter || undefined,
				status: statusFilter || undefined,
			});
			setBranches(data);
			setError(null);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load branches');
		} finally {
			setLoading(false);
		}
	}, [typeFilter, statusFilter]);

	// Initial load and reload on filter change
	useEffect(() => {
		loadBranches();
	}, [loadBranches]);

	// Count branches by status
	const statusCounts = useMemo(() => {
		const counts = { active: 0, merged: 0, stale: 0, orphaned: 0, total: 0 };
		branches.forEach((b) => {
			counts[b.status]++;
			counts.total++;
		});
		return counts;
	}, [branches]);

	// Handle status change
	const handleStatusChange = useCallback(
		async (name: string, newStatus: BranchStatus) => {
			setActionLoading(name);
			try {
				await updateBranchStatus(name, newStatus);
				toast.success(`Branch status updated to ${newStatus}`);
				loadBranches();
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Failed to update status');
			} finally {
				setActionLoading(null);
			}
		},
		[loadBranches]
	);

	// Handle branch deletion
	const handleDelete = useCallback(
		async (name: string) => {
			if (!confirm(`Delete branch "${name}" from tracking registry?\n\nThis removes the tracking entry only, not the actual git branch.`)) {
				return;
			}
			setActionLoading(name);
			try {
				await deleteBranch(name);
				toast.success('Branch removed from registry');
				loadBranches();
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Failed to delete branch');
			} finally {
				setActionLoading(null);
			}
		},
		[loadBranches]
	);

	// Cleanup merged/orphaned branches
	const handleCleanup = useCallback(async () => {
		const toClean = branches.filter((b) => b.status === 'merged' || b.status === 'orphaned');
		if (toClean.length === 0) {
			toast.info('No branches to clean up');
			return;
		}
		if (
			!confirm(
				`Clean up ${toClean.length} branch(es)?\n\n${toClean.map((b) => `• ${b.name} (${b.status})`).join('\n')}\n\nThis removes tracking entries only, not actual git branches.`
			)
		) {
			return;
		}

		let cleaned = 0;
		for (const branch of toClean) {
			try {
				await deleteBranch(branch.name);
				cleaned++;
			} catch {
				// Continue on error
			}
		}
		toast.success(`Cleaned up ${cleaned} branch(es)`);
		loadBranches();
	}, [branches, loadBranches]);

	// Get owner link based on branch type
	const getOwnerLink = (branch: Branch) => {
		if (!branch.owner_id) return null;
		if (branch.type === 'initiative') {
			return <Link to={`/initiatives/${branch.owner_id}`}>{branch.owner_id}</Link>;
		}
		if (branch.type === 'task') {
			return <Link to={`/tasks/${branch.owner_id}`}>{branch.owner_id}</Link>;
		}
		return <span>{branch.owner_id}</span>;
	};

	return (
		<div className="page branches-page">
			<header className="page-header">
				<div className="header-title">
					<h1>Branches</h1>
					<span className="branch-count">{statusCounts.total} tracked</span>
				</div>
				<div className="header-actions">
					<Button
						variant="secondary"
						size="sm"
						leftIcon={<Icon name="rotate-ccw" size={14} />}
						onClick={loadBranches}
						disabled={loading}
					>
						Refresh
					</Button>
					<Button
						variant="danger"
						size="sm"
						leftIcon={<Icon name="trash" size={14} />}
						onClick={handleCleanup}
						disabled={loading || (statusCounts.merged + statusCounts.orphaned === 0)}
					>
						Cleanup ({statusCounts.merged + statusCounts.orphaned})
					</Button>
				</div>
			</header>

			{/* Status summary */}
			<div className="status-summary">
				{Object.entries(BRANCH_STATUS_CONFIG).map(([status, config]) => (
					<div
						key={status}
						className={`status-card ${statusFilter === status ? 'active' : ''}`}
						onClick={() => setStatusFilter(statusFilter === status ? '' : (status as BranchStatus))}
					>
						<span className="status-dot" style={{ backgroundColor: config.color }} />
						<span className="status-label">{config.label}</span>
						<span className="status-count">{statusCounts[status as BranchStatus]}</span>
					</div>
				))}
			</div>

			{/* Filters */}
			<div className="filters">
				<div className="filter-group">
					<label htmlFor="type-filter">Type</label>
					<select
						id="type-filter"
						value={typeFilter}
						onChange={(e) => setTypeFilter(e.target.value as BranchType | '')}
					>
						{BRANCH_TYPES.map((opt) => (
							<option key={opt.value} value={opt.value}>
								{opt.label}
							</option>
						))}
					</select>
				</div>
				<div className="filter-group">
					<label htmlFor="status-filter">Status</label>
					<select
						id="status-filter"
						value={statusFilter}
						onChange={(e) => setStatusFilter(e.target.value as BranchStatus | '')}
					>
						{BRANCH_STATUSES.map((opt) => (
							<option key={opt.value} value={opt.value}>
								{opt.label}
							</option>
						))}
					</select>
				</div>
			</div>

			{/* Content */}
			{loading ? (
				<div className="loading-state">Loading branches...</div>
			) : error ? (
				<div className="error-state">
					<Icon name="error" size={24} />
					<p>{error}</p>
					<Button variant="secondary" size="sm" onClick={loadBranches}>
						Retry
					</Button>
				</div>
			) : branches.length === 0 ? (
				<div className="empty-state">
					<Icon name="git-branch" size={48} />
					<h3>No branches tracked</h3>
					<p>
						{typeFilter || statusFilter
							? 'No branches match the current filters.'
							: 'Orc will track branches as you create initiatives or run tasks.'}
					</p>
				</div>
			) : (
				<div className="branch-list">
					<table>
						<thead>
							<tr>
								<th>Branch</th>
								<th>Type</th>
								<th>Owner</th>
								<th>Status</th>
								<th>Last Activity</th>
								<th>Actions</th>
							</tr>
						</thead>
						<tbody>
							{branches.map((branch) => {
								const typeConfig = BRANCH_TYPE_CONFIG[branch.type];
								const statusConfig = BRANCH_STATUS_CONFIG[branch.status];
								const isLoading = actionLoading === branch.name;

								return (
									<tr key={branch.name} className={isLoading ? 'loading' : ''}>
										<td className="branch-name">
											<Icon name="git-branch" size={14} />
											<span title={branch.name}>{branch.name}</span>
										</td>
										<td>
											<span className="type-badge">
												<Icon name={typeConfig.icon as 'flag' | 'git-branch' | 'check-square'} size={12} />
												{typeConfig.label}
											</span>
										</td>
										<td className="owner-cell">{getOwnerLink(branch) || '—'}</td>
										<td>
											<span
												className="status-badge"
												style={{ '--status-color': statusConfig.color } as React.CSSProperties}
											>
												{statusConfig.label}
											</span>
										</td>
										<td className="activity-cell">
											<span title={new Date(branch.last_activity).toLocaleString()}>
												{formatTimeAgo(branch.last_activity)}
											</span>
										</td>
										<td className="actions-cell">
											{branch.status === 'active' && (
												<Button
													variant="ghost"
													size="sm"
													iconOnly
													title="Mark as stale"
													aria-label="Mark as stale"
													onClick={() => handleStatusChange(branch.name, 'stale')}
													disabled={isLoading}
												>
													<Icon name="clock" size={14} />
												</Button>
											)}
											{branch.status === 'stale' && (
												<Button
													variant="ghost"
													size="sm"
													iconOnly
													title="Mark as active"
													aria-label="Mark as active"
													onClick={() => handleStatusChange(branch.name, 'active')}
													disabled={isLoading}
												>
													<Icon name="check" size={14} />
												</Button>
											)}
											<Button
												variant="ghost"
												size="sm"
												iconOnly
												title="Remove from registry"
												aria-label="Remove from registry"
												onClick={() => handleDelete(branch.name)}
												disabled={isLoading}
											>
												<Icon name="trash" size={14} />
											</Button>
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			)}
		</div>
	);
}
