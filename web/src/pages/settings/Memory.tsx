/**
 * Memory page (/settings/memory)
 * Displays and manages knowledge entries (patterns, gotchas, decisions)
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon, type IconName } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { knowledgeClient } from '@/lib/client';
import {
	ListKnowledgeRequestSchema,
	CreateKnowledgeRequestSchema,
	ApproveKnowledgeRequestSchema,
	RejectKnowledgeRequestSchema,
	DeleteKnowledgeRequestSchema,
	ApproveAllKnowledgeRequestSchema,
	KnowledgeType,
	KnowledgeStatus,
	type KnowledgeEntry,
	type KnowledgeStatusSummary,
} from '@/gen/orc/v1/knowledge_pb';
import './Memory.css';

// Map enum values to display strings
const typeLabels: Record<KnowledgeType, string> = {
	[KnowledgeType.UNSPECIFIED]: 'Unknown',
	[KnowledgeType.PATTERN]: 'Pattern',
	[KnowledgeType.GOTCHA]: 'Gotcha',
	[KnowledgeType.DECISION]: 'Decision',
};

const statusLabels: Record<KnowledgeStatus, string> = {
	[KnowledgeStatus.UNSPECIFIED]: 'Unknown',
	[KnowledgeStatus.PENDING]: 'Pending',
	[KnowledgeStatus.APPROVED]: 'Approved',
	[KnowledgeStatus.REJECTED]: 'Rejected',
	[KnowledgeStatus.STALE]: 'Stale',
};

const typeIcons: Record<KnowledgeType, IconName> = {
	[KnowledgeType.UNSPECIFIED]: 'info',
	[KnowledgeType.PATTERN]: 'code',
	[KnowledgeType.GOTCHA]: 'alert-triangle',
	[KnowledgeType.DECISION]: 'check-circle',
};

interface MemoryFormData {
	type: KnowledgeType;
	name: string;
	description: string;
	proposedBy: string;
}

export function Memory() {
	useDocumentTitle('Memory');
	const [entries, setEntries] = useState<KnowledgeEntry[]>([]);
	const [summary, setSummary] = useState<KnowledgeStatusSummary | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Filter state
	const [filterStatus, setFilterStatus] = useState<KnowledgeStatus | null>(null);
	const [filterType, setFilterType] = useState<KnowledgeType | null>(null);

	// Create modal state
	const [showCreateModal, setShowCreateModal] = useState(false);
	const [formData, setFormData] = useState<MemoryFormData>({
		type: KnowledgeType.PATTERN,
		name: '',
		description: '',
		proposedBy: 'user',
	});
	const [saving, setSaving] = useState(false);

	// Reject modal state
	const [rejectingEntry, setRejectingEntry] = useState<KnowledgeEntry | null>(null);
	const [rejectReason, setRejectReason] = useState('');

	const loadData = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);

			// Load entries and status summary in parallel
			const [entriesRes, statusRes] = await Promise.all([
				knowledgeClient.listKnowledge(
					create(ListKnowledgeRequestSchema, {
						type: filterType ?? undefined,
						status: filterStatus ?? undefined,
					})
				),
				knowledgeClient.getKnowledgeStatus({}),
			]);

			setEntries(entriesRes.entries);
			setSummary(statusRes.status ?? null);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load memory entries');
		} finally {
			setLoading(false);
		}
	}, [filterStatus, filterType]);

	useEffect(() => {
		loadData();
	}, [loadData]);

	const handleCreate = async () => {
		if (!formData.name?.trim()) {
			toast.error('Name is required');
			return;
		}
		if (!formData.description?.trim()) {
			toast.error('Description is required');
			return;
		}

		try {
			setSaving(true);
			await knowledgeClient.createKnowledge(
				create(CreateKnowledgeRequestSchema, {
					type: formData.type,
					name: formData.name.trim(),
					description: formData.description.trim(),
					proposedBy: formData.proposedBy || 'user',
				})
			);
			toast.success('Memory entry created');
			setShowCreateModal(false);
			setFormData({
				type: KnowledgeType.PATTERN,
				name: '',
				description: '',
				proposedBy: 'user',
			});
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create entry');
		} finally {
			setSaving(false);
		}
	};

	const handleApprove = async (entry: KnowledgeEntry) => {
		try {
			await knowledgeClient.approveKnowledge(
				create(ApproveKnowledgeRequestSchema, {
					id: entry.id,
					reviewedBy: 'user',
				})
			);
			toast.success(`Approved: ${entry.name}`);
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to approve');
		}
	};

	const handleReject = async () => {
		if (!rejectingEntry) return;

		try {
			await knowledgeClient.rejectKnowledge(
				create(RejectKnowledgeRequestSchema, {
					id: rejectingEntry.id,
					reason: rejectReason || undefined,
					reviewedBy: 'user',
				})
			);
			toast.success(`Rejected: ${rejectingEntry.name}`);
			setRejectingEntry(null);
			setRejectReason('');
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to reject');
		}
	};

	const handleDelete = async (entry: KnowledgeEntry) => {
		if (!confirm(`Delete "${entry.name}"?`)) return;

		try {
			await knowledgeClient.deleteKnowledge(
				create(DeleteKnowledgeRequestSchema, { id: entry.id })
			);
			toast.success('Entry deleted');
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete');
		}
	};

	const handleApproveAll = async () => {
		if (!summary?.pendingCount || summary.pendingCount === 0) return;
		if (!confirm(`Approve all ${summary.pendingCount} pending entries?`)) return;

		try {
			const res = await knowledgeClient.approveAllKnowledge(
				create(ApproveAllKnowledgeRequestSchema, { reviewedBy: 'user' })
			);
			toast.success(`Approved ${res.approvedCount} entries`);
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to approve all');
		}
	};

	const formatDate = (timestamp: { seconds?: bigint } | undefined) => {
		if (!timestamp?.seconds) return '';
		const date = new Date(Number(timestamp.seconds) * 1000);
		return date.toLocaleDateString();
	};

	if (loading) {
		return (
			<div className="page memory-page">
				<div className="env-loading">Loading memory entries...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page memory-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadData}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page memory-page">
			<div className="env-page-header">
				<div>
					<h3>Memory</h3>
					<p className="env-page-description">
						Manage Claude's persistent memory: patterns, gotchas, and architectural decisions.
					</p>
				</div>
				<div className="env-page-header-actions">
					{summary && summary.pendingCount > 0 && (
						<Button variant="secondary" onClick={handleApproveAll}>
							<Icon name="check-circle" size={14} />
							Approve All ({summary.pendingCount})
						</Button>
					)}
					<Button variant="primary" onClick={() => setShowCreateModal(true)}>
						<Icon name="plus" size={14} />
						Add Entry
					</Button>
				</div>
			</div>

			{/* Status Summary */}
			{summary && (
				<div className="memory-summary">
					<button
						className={`memory-stat ${filterStatus === KnowledgeStatus.PENDING ? 'active' : ''}`}
						onClick={() =>
							setFilterStatus(
								filterStatus === KnowledgeStatus.PENDING ? null : KnowledgeStatus.PENDING
							)
						}
					>
						<span className="memory-stat-count">{summary.pendingCount}</span>
						<span className="memory-stat-label">Pending</span>
					</button>
					<button
						className={`memory-stat ${filterStatus === KnowledgeStatus.APPROVED ? 'active' : ''}`}
						onClick={() =>
							setFilterStatus(
								filterStatus === KnowledgeStatus.APPROVED ? null : KnowledgeStatus.APPROVED
							)
						}
					>
						<span className="memory-stat-count">{summary.approvedCount}</span>
						<span className="memory-stat-label">Approved</span>
					</button>
					<button
						className={`memory-stat ${filterStatus === KnowledgeStatus.STALE ? 'active' : ''}`}
						onClick={() =>
							setFilterStatus(
								filterStatus === KnowledgeStatus.STALE ? null : KnowledgeStatus.STALE
							)
						}
					>
						<span className="memory-stat-count">{summary.staleCount}</span>
						<span className="memory-stat-label">Stale</span>
					</button>
					<button
						className={`memory-stat ${filterStatus === KnowledgeStatus.REJECTED ? 'active' : ''}`}
						onClick={() =>
							setFilterStatus(
								filterStatus === KnowledgeStatus.REJECTED ? null : KnowledgeStatus.REJECTED
							)
						}
					>
						<span className="memory-stat-count">{summary.rejectedCount}</span>
						<span className="memory-stat-label">Rejected</span>
					</button>
				</div>
			)}

			{/* Type Filter */}
			<div className="memory-filters">
				<span className="memory-filters-label">Type:</span>
				<button
					className={`memory-filter-btn ${filterType === null ? 'active' : ''}`}
					onClick={() => setFilterType(null)}
				>
					All
				</button>
				<button
					className={`memory-filter-btn ${filterType === KnowledgeType.PATTERN ? 'active' : ''}`}
					onClick={() =>
						setFilterType(filterType === KnowledgeType.PATTERN ? null : KnowledgeType.PATTERN)
					}
				>
					<Icon name="code" size={12} />
					Patterns
				</button>
				<button
					className={`memory-filter-btn ${filterType === KnowledgeType.GOTCHA ? 'active' : ''}`}
					onClick={() =>
						setFilterType(filterType === KnowledgeType.GOTCHA ? null : KnowledgeType.GOTCHA)
					}
				>
					<Icon name="alert-triangle" size={12} />
					Gotchas
				</button>
				<button
					className={`memory-filter-btn ${filterType === KnowledgeType.DECISION ? 'active' : ''}`}
					onClick={() =>
						setFilterType(filterType === KnowledgeType.DECISION ? null : KnowledgeType.DECISION)
					}
				>
					<Icon name="check-circle" size={12} />
					Decisions
				</button>
			</div>

			{/* Entries List */}
			{entries.length === 0 ? (
				<div className="env-empty">
					<Icon name="database" size={48} />
					<p>
						{filterStatus || filterType
							? 'No entries match the current filters'
							: 'No memory entries yet'}
					</p>
					<p className="env-empty-hint">
						{filterStatus || filterType
							? 'Try adjusting your filters or add a new entry.'
							: 'Add patterns, gotchas, and decisions to help Claude understand your codebase.'}
					</p>
				</div>
			) : (
				<div className="memory-list">
					{entries.map((entry) => (
						<div key={entry.id} className={`memory-card status-${entry.status}`}>
							<div className="memory-card-header">
								<div className="memory-card-title">
									<Icon name={typeIcons[entry.type]} size={16} />
									<span>{entry.name}</span>
								</div>
								<div className="memory-card-actions">
									{entry.status === KnowledgeStatus.PENDING && (
										<>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => handleApprove(entry)}
												aria-label="Approve"
											>
												<Icon name="check" size={14} />
											</Button>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => setRejectingEntry(entry)}
												aria-label="Reject"
											>
												<Icon name="x" size={14} />
											</Button>
										</>
									)}
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handleDelete(entry)}
										aria-label="Delete"
									>
										<Icon name="trash" size={14} />
									</Button>
								</div>
							</div>

							<div className="memory-card-description">{entry.description}</div>

							<div className="memory-card-meta">
								<span className={`memory-badge type-${entry.type}`}>
									{typeLabels[entry.type]}
								</span>
								<span className={`memory-badge status-${entry.status}`}>
									{statusLabels[entry.status]}
								</span>
								{entry.sourceTask && (
									<span className="memory-badge source">From: {entry.sourceTask}</span>
								)}
								{entry.proposedBy && (
									<span className="memory-meta-text">by {entry.proposedBy}</span>
								)}
								{entry.createdAt && (
									<span className="memory-meta-text">{formatDate(entry.createdAt)}</span>
								)}
							</div>

							{entry.reviewReason && (
								<div className="memory-card-review">
									<Icon name="message-circle" size={12} />
									{entry.reviewReason}
								</div>
							)}
						</div>
					))}
				</div>
			)}

			{/* Create Modal */}
			<Modal
				open={showCreateModal}
				onClose={() => setShowCreateModal(false)}
				title="Add Memory Entry"
				size="md"
			>
				<div className="memory-form">
					<div className="memory-form-field">
						<label>Type</label>
						<select
							value={formData.type}
							onChange={(e) =>
								setFormData({ ...formData, type: Number(e.target.value) as KnowledgeType })
							}
							className="input-field"
						>
							<option value={KnowledgeType.PATTERN}>Pattern - Code pattern or convention</option>
							<option value={KnowledgeType.GOTCHA}>Gotcha - Pitfall or common mistake</option>
							<option value={KnowledgeType.DECISION}>
								Decision - Architectural decision
							</option>
						</select>
					</div>

					<div className="memory-form-field">
						<label>Name</label>
						<Input
							value={formData.name}
							onChange={(e) => setFormData({ ...formData, name: e.target.value })}
							placeholder="Short descriptive name"
							size="sm"
						/>
					</div>

					<div className="memory-form-field">
						<label>Description</label>
						<textarea
							value={formData.description}
							onChange={(e) => setFormData({ ...formData, description: e.target.value })}
							className="textarea-field memory-description-textarea"
							placeholder="Detailed description of the pattern, gotcha, or decision..."
							rows={5}
						/>
					</div>

					<div className="memory-form-actions">
						<Button variant="secondary" onClick={() => setShowCreateModal(false)}>
							Cancel
						</Button>
						<Button variant="primary" onClick={handleCreate} loading={saving}>
							Create Entry
						</Button>
					</div>
				</div>
			</Modal>

			{/* Reject Modal */}
			<Modal
				open={rejectingEntry !== null}
				onClose={() => {
					setRejectingEntry(null);
					setRejectReason('');
				}}
				title={`Reject: ${rejectingEntry?.name}`}
				size="sm"
			>
				<div className="memory-form">
					<div className="memory-form-field">
						<label>Reason (optional)</label>
						<textarea
							value={rejectReason}
							onChange={(e) => setRejectReason(e.target.value)}
							className="textarea-field"
							placeholder="Why is this entry being rejected?"
							rows={3}
						/>
					</div>

					<div className="memory-form-actions">
						<Button
							variant="secondary"
							onClick={() => {
								setRejectingEntry(null);
								setRejectReason('');
							}}
						>
							Cancel
						</Button>
						<Button variant="danger" onClick={handleReject}>
							Reject Entry
						</Button>
					</div>
				</div>
			</Modal>
		</div>
	);
}
