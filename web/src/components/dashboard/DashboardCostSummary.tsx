import { useState, useEffect } from 'react';
import { dashboardClient } from '@/lib/client';

interface DashboardCostSummaryProps {
	projectId: string;
}

interface CostData {
	totalCostUsd: number;
	breakdowns: Array<{ key: string; costUsd: number }>;
	budgetLimitUsd?: number;
	budgetPercentUsed?: number;
}

export function DashboardCostSummary({ projectId }: DashboardCostSummaryProps) {
	const [data, setData] = useState<CostData | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [groupBy, setGroupBy] = useState<'model' | 'provider'>('model');

	useEffect(() => {
		let cancelled = false;
		setLoading(true);

		async function fetchCosts() {
			try {
				const resp = await dashboardClient.getCostReport({
					projectId,
					groupBy,
				});
				if (!cancelled) {
					setData(resp);
					setLoading(false);
				}
			} catch {
				if (!cancelled) {
					setError('Failed to load cost data');
					setLoading(false);
				}
			}
		}

		fetchCosts();
		return () => { cancelled = true; };
	}, [projectId, groupBy]);

	if (loading) {
		return (
			<section data-testid="cost-summary" className="cost-summary-section">
				<span data-testid="cost-loading">Loading costs...</span>
			</section>
		);
	}

	if (error) {
		return (
			<section data-testid="cost-summary" className="cost-summary-section">
				<span className="cost-error">{error}</span>
			</section>
		);
	}

	const total = data?.totalCostUsd ?? 0;
	const breakdowns = data?.breakdowns ?? [];

	return (
		<section data-testid="cost-summary" className="cost-summary-section">
			<h3>Cost Summary</h3>
			<div className="cost-total">
				<span className="cost-label">Current Month</span>
				<span className="cost-value">${total.toFixed(2)}</span>
			</div>
			{data?.budgetPercentUsed != null && (
				<div className="cost-budget">
					<span className="cost-label">Budget Used</span>
					<span className="cost-value">{Math.round(data.budgetPercentUsed)}%</span>
				</div>
			)}
			{breakdowns.length > 0 && (
				<div className="cost-breakdowns">
					<div style={{ display: 'flex', gap: '4px', marginBottom: '8px' }}>
						<button
							className={`cost-group-btn ${groupBy === 'model' ? 'active' : ''}`}
							onClick={() => setGroupBy('model')}
							style={{
								fontSize: '0.75rem',
								padding: '2px 8px',
								border: '1px solid var(--color-border, #444)',
								borderRadius: '4px',
								background: groupBy === 'model' ? 'var(--color-accent, #666)' : 'transparent',
								color: 'inherit',
								cursor: 'pointer',
							}}
						>
							By Model
						</button>
						<button
							className={`cost-group-btn ${groupBy === 'provider' ? 'active' : ''}`}
							onClick={() => setGroupBy('provider')}
							style={{
								fontSize: '0.75rem',
								padding: '2px 8px',
								border: '1px solid var(--color-border, #444)',
								borderRadius: '4px',
								background: groupBy === 'provider' ? 'var(--color-accent, #666)' : 'transparent',
								color: 'inherit',
								cursor: 'pointer',
							}}
						>
							By Provider
						</button>
					</div>
					{breakdowns.map((b) => (
						<div key={b.key} className="cost-breakdown-item">
							<span className="cost-breakdown-key">{b.key}</span>
							<span className="cost-breakdown-value">${b.costUsd.toFixed(2)}</span>
						</div>
					))}
				</div>
			)}
		</section>
	);
}
