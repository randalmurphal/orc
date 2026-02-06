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

	useEffect(() => {
		let cancelled = false;

		async function fetchCosts() {
			try {
				const resp = await dashboardClient.getCostReport({
					projectId,
					groupBy: 'model',
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
	}, [projectId]);

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
