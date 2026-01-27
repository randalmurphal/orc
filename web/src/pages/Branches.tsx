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
import { branchClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { useDocumentTitle } from '@/hooks';
import type { Branch } from '@/gen/orc/v1/project_pb';
import { BranchType, BranchStatus } from '@/gen/orc/v1/project_pb';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import './Branches.css';

// Display config for branch status
const BRANCH_STATUS_CONFIG: Record<BranchStatus, { label: string; color: string }> = {
	[BranchStatus.UNSPECIFIED]: { label: 'Unknown', color: 'var(--text-muted)' },
	[BranchStatus.ACTIVE]: { label: 'Active', color: 'var(--status-success)' },
	[BranchStatus.MERGED]: { label: 'Merged', color: 'var(--status-info)' },
	[BranchStatus.STALE]: { label: 'Stale', color: 'var(--status-warning)' },
	[BranchStatus.ORPHANED]: { label: 'Orphaned', color: 'var(--status-error)' },
};

// Display config for branch type
const BRANCH_TYPE_CONFIG: Record<BranchType, { label: string; icon: string }> = {
	[BranchType.UNSPECIFIED]: { label: 'Unknown', icon: 'help-circle' },
	[BranchType.INITIATIVE]: { label: 'Initiative', icon: 'layers' },
	[BranchType.STAGING]: { label: 'Staging', icon: 'git-branch' },
	[BranchType.TASK]: { label: 'Task', icon: 'check-circle' },
};

// Filter options
const BRANCH_TYPES: { value: BranchType | undefined; label: string }[] = [
	{ value: undefined, label: 'All Types' },
	{ value: BranchType.INITIATIVE, label: 'Initiative' },
	{ value: BranchType.STAGING, label: 'Staging' },
	{ value: BranchType.TASK, label: 'Task' },
];

const BRANCH_STATUSES: { value: BranchStatus | undefined; label: string }[] = [
	{ value: undefined, label: 'All Statuses' },
	{ value: BranchStatus.ACTIVE, label: 'Active' },
	{ value: BranchStatus.MERGED, label: 'Merged' },
	{ value: BranchStatus.STALE, label: 'Stale' },
	{ value: BranchStatus.ORPHANED, label: 'Orphaned' },
];

// Status display entries for the summary bar
const STATUS_DISPLAY_ENTRIES: { status: BranchStatus; config: { label: string; color: string } }[] = [
	{ status: BranchStatus.ACTIVE, config: BRANCH_STATUS_CONFIG[BranchStatus.ACTIVE] },
	{ status: BranchStatus.MERGED, config: BRANCH_STATUS_CONFIG[BranchStatus.MERGED] },
	{ status: BranchStatus.STALE, config: BRANCH_STATUS_CONFIG[BranchStatus.STALE] },
	{ status: BranchStatus.ORPHANED, config: BRANCH_STATUS_CONFIG[BranchStatus.ORPHANED] },
];

// Convert Timestamp to Date
function timestampToDate(ts: Timestamp | undefined): Date {
	if (!ts) return new Date(0);
	// Timestamp has seconds (bigint) and nanos
	return new Date(Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000));
}

// Format relative time
function formatTimeAgo(ts: Timestamp | undefined): string {
	const date = timestampToDate(ts);
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
	useDocumentTitle('Branches');
	const [branches, setBranches] = useState<Branch[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [typeFilter, setTypeFilter] = useState<BranchType | undefined>(undefined);
	const [statusFilter, setStatusFilter] = useState<BranchStatus | undefined>(undefined);
	const [actionLoading, setActionLoading] = useState<string | null>(null);

	const loadBranches = useCallback(async () => {
		try {
			setLoading(true);
			const response = await branchClient.listBranches({
				type: typeFilter,
				status: statusFilter,
			});
			setBranches(response.branches);
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
		const counts: Record<BranchStatus, number> & { total: number } = {
			[BranchStatus.UNSPECIFIED]: 0,
			[BranchStatus.ACTIVE]: 0,
			[BranchStatus.MERGED]: 0,
			[BranchStatus.STALE]: 0,
			[BranchStatus.ORPHANED]: 0,
			total: 0,
		};
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
				await branchClient.updateBranchStatus({ name, status: newStatus });
				toast.success(`Branch status updated to ${BRANCH_STATUS_CONFIG[newStatus].label}`);
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
				await branchClient.deleteBranch({ name });
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
		const toClean = branches.filter((b) => b.status === BranchStatus.MERGED || b.status === BranchStatus.ORPHANED);
		if (toClean.length === 0) {
			toast.info('No branches to clean up');
			return;
		}
		if (
			!confirm(
				`Clean up ${toClean.length} branch(es)?\n\n${toClean.map((b) => `- ${b.name} (${BRANCH_STATUS_CONFIG[b.status].label})`).join('\n')}\n\nThis removes tracking entries only, not actual git branches.`
			)
		) {
			return;
		}

		let cleaned = 0;
		for (const branch of toClean) {
			try {
				await branchClient.deleteBranch({ name: branch.name });
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
		if (!branch.ownerId) return null;
		if (branch.type === BranchType.INITIATIVE) {
			return <Link to={`/initiatives/${branch.ownerId}`}>{branch.ownerId}</Link>;
		}
		if (branch.type === BranchType.TASK) {
			return <Link to={`/tasks/${branch.ownerId}`}>{branch.ownerId}</Link>;
		}
		return <span>{branch.ownerId}</span>;
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
						disabled={loading || (statusCounts[BranchStatus.MERGED] + statusCounts[BranchStatus.ORPHANED] === 0)}
					>
						Cleanup ({statusCounts[BranchStatus.MERGED] + statusCounts[BranchStatus.ORPHANED]})
					</Button>
				</div>
			</header>

			{/* Status summary */}
			<div className="status-summary">
				{STATUS_DISPLAY_ENTRIES.map(({ status, config }) => (
					<div
						key={status}
						className={`status-card ${statusFilter === status ? 'active' : ''}`}
						onClick={() => setStatusFilter(statusFilter === status ? undefined : status)}
					>
						<span className="status-dot" style={{ backgroundColor: config.color }} />
						<span className="status-label">{config.label}</span>
						<span className="status-count">{statusCounts[status]}</span>
					</div>
				))}
			</div>

			{/* Filters */}
			<div className="filters">
				<div className="filter-group">
					<label htmlFor="type-filter">Type</label>
					<select
						id="type-filter"
						value={typeFilter ?? ''}
						onChange={(e) => setTypeFilter(e.target.value ? Number(e.target.value) as BranchType : undefined)}
					>
						{BRANCH_TYPES.map((opt) => (
							<option key={opt.value ?? 'all'} value={opt.value ?? ''}>
								{opt.label}
							</option>
						))}
					</select>
				</div>
				<div className="filter-group">
					<label htmlFor="status-filter">Status</label>
					<select
						id="status-filter"
						value={statusFilter ?? ''}
						onChange={(e) => setStatusFilter(e.target.value ? Number(e.target.value) as BranchStatus : undefined)}
					>
						{BRANCH_STATUSES.map((opt) => (
							<option key={opt.value ?? 'all'} value={opt.value ?? ''}>
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
						{typeFilter !== undefined || statusFilter !== undefined
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
												<Icon name={typeConfig.icon as 'layers' | 'git-branch' | 'check-circle'} size={12} />
												{typeConfig.label}
											</span>
										</td>
										<td className="owner-cell">{getOwnerLink(branch) || '-'}</td>
										<td>
											<span
												className="status-badge"
												style={{ '--status-color': statusConfig.color } as React.CSSProperties}
											>
												{statusConfig.label}
											</span>
										</td>
										<td className="activity-cell">
											<span title={timestampToDate(branch.lastActivity).toLocaleString()}>
												{formatTimeAgo(branch.lastActivity)}
											</span>
										</td>
										<td className="actions-cell">
											{branch.status === BranchStatus.ACTIVE && (
												<Button
													variant="ghost"
													size="sm"
													iconOnly
													title="Mark as stale"
													aria-label="Mark as stale"
													onClick={() => handleStatusChange(branch.name, BranchStatus.STALE)}
													disabled={isLoading}
												>
													<Icon name="clock" size={14} />
												</Button>
											)}
											{branch.status === BranchStatus.STALE && (
												<Button
													variant="ghost"
													size="sm"
													iconOnly
													title="Mark as active"
													aria-label="Mark as active"
													onClick={() => handleStatusChange(branch.name, BranchStatus.ACTIVE)}
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
