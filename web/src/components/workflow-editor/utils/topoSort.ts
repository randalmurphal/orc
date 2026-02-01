/**
 * Topological sort for workflow phases.
 *
 * Assigns sequence numbers based on dependency topology using Kahn's algorithm.
 * Phases at the same dependency level get the same sequence number,
 * enabling parallel execution.
 *
 * @param phases - Array of phase info with id and dependencies
 * @returns Map from phase id to sequence number (1-based)
 */

export interface PhaseForSort {
	id: string;
	dependsOn: string[];
}

export function topoSort(phases: PhaseForSort[]): Map<string, number> {
	const phaseIds = new Set(phases.map((p) => p.id));

	// Build in-degree counts and adjacency list
	const inDegree = new Map<string, number>();
	const dependents = new Map<string, string[]>();

	for (const phase of phases) {
		// Only count dependencies on phases that exist in this workflow
		const validDeps = phase.dependsOn.filter((d) => phaseIds.has(d));
		inDegree.set(phase.id, validDeps.length);

		for (const dep of validDeps) {
			const list = dependents.get(dep) ?? [];
			list.push(phase.id);
			dependents.set(dep, list);
		}
	}

	// Initialize with phases that have no dependencies
	let queue = phases
		.filter((p) => (inDegree.get(p.id) ?? 0) === 0)
		.map((p) => p.id);

	const result = new Map<string, number>();
	let sequence = 1;

	while (queue.length > 0) {
		// All phases in the current queue are at the same dependency level
		for (const id of queue) {
			result.set(id, sequence);
		}

		const nextQueue: string[] = [];
		for (const id of queue) {
			for (const dependent of dependents.get(id) ?? []) {
				const newDegree = (inDegree.get(dependent) ?? 1) - 1;
				inDegree.set(dependent, newDegree);
				if (newDegree === 0) {
					nextQueue.push(dependent);
				}
			}
		}

		queue = nextQueue;
		sequence++;
	}

	// Phases not reached (cycles or orphans) get sequence after all others
	for (const phase of phases) {
		if (!result.has(phase.id)) {
			result.set(phase.id, sequence);
		}
	}

	return result;
}
