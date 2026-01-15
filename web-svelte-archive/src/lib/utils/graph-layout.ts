/**
 * Simple DAG layout algorithm for dependency graphs.
 * Uses topological sorting to assign layers, then positions nodes within each layer.
 */

export interface LayoutNode {
	id: string;
	x: number;
	y: number;
	width: number;
	height: number;
	layer: number;
}

export interface LayoutEdge {
	from: string;
	to: string;
	points: Array<{ x: number; y: number }>;
}

export interface LayoutConfig {
	nodeWidth: number;
	nodeHeight: number;
	horizontalSpacing: number;
	verticalSpacing: number;
	padding: number;
}

export interface LayoutResult {
	nodes: Map<string, LayoutNode>;
	edges: LayoutEdge[];
	width: number;
	height: number;
}

const defaultConfig: LayoutConfig = {
	nodeWidth: 120,
	nodeHeight: 60,
	horizontalSpacing: 40,
	verticalSpacing: 80,
	padding: 40
};

/**
 * Compute layout for a directed acyclic graph.
 * Nodes with no dependencies are placed at the top.
 * Leaf nodes (nothing depends on them) are at the bottom.
 */
export function computeLayout(
	nodes: Array<{ id: string }>,
	edges: Array<{ from: string; to: string }>,
	config: Partial<LayoutConfig> = {}
): LayoutResult {
	const cfg = { ...defaultConfig, ...config };

	// Build adjacency lists
	const inDegree = new Map<string, number>();
	const outEdges = new Map<string, string[]>();
	const inEdges = new Map<string, string[]>();

	// Initialize
	for (const node of nodes) {
		inDegree.set(node.id, 0);
		outEdges.set(node.id, []);
		inEdges.set(node.id, []);
	}

	// Build graph structure
	for (const edge of edges) {
		// edge.from blocks edge.to (from -> to means "to depends on from")
		const fromEdges = outEdges.get(edge.from);
		if (fromEdges) {
			fromEdges.push(edge.to);
		}

		const toEdges = inEdges.get(edge.to);
		if (toEdges) {
			toEdges.push(edge.from);
		}

		const deg = inDegree.get(edge.to) ?? 0;
		inDegree.set(edge.to, deg + 1);
	}

	// Assign layers using topological sort (Kahn's algorithm)
	const layers: string[][] = [];
	const nodeLayer = new Map<string, number>();
	const remaining = new Map(inDegree);

	while (remaining.size > 0) {
		// Find nodes with no remaining incoming edges
		const currentLayer: string[] = [];
		for (const [id, degree] of remaining) {
			if (degree === 0) {
				currentLayer.push(id);
			}
		}

		if (currentLayer.length === 0) {
			// Cycle detected - just place remaining nodes in current layer
			for (const [id] of remaining) {
				currentLayer.push(id);
			}
		}

		// Sort nodes in layer for consistent ordering
		currentLayer.sort();

		const layerIndex = layers.length;
		layers.push(currentLayer);

		// Assign layer to each node
		for (const id of currentLayer) {
			nodeLayer.set(id, layerIndex);
			remaining.delete(id);

			// Decrease in-degree of neighbors
			const neighbors = outEdges.get(id) ?? [];
			for (const neighbor of neighbors) {
				const deg = remaining.get(neighbor);
				if (deg !== undefined) {
					remaining.set(neighbor, deg - 1);
				}
			}
		}
	}

	// Position nodes within each layer
	const layoutNodes = new Map<string, LayoutNode>();
	let maxWidth = 0;

	for (let layerIndex = 0; layerIndex < layers.length; layerIndex++) {
		const layer = layers[layerIndex];
		const layerWidth = layer.length * (cfg.nodeWidth + cfg.horizontalSpacing) - cfg.horizontalSpacing;
		maxWidth = Math.max(maxWidth, layerWidth);
	}

	// Center layers horizontally
	for (let layerIndex = 0; layerIndex < layers.length; layerIndex++) {
		const layer = layers[layerIndex];
		const layerWidth = layer.length * (cfg.nodeWidth + cfg.horizontalSpacing) - cfg.horizontalSpacing;
		const startX = cfg.padding + (maxWidth - layerWidth) / 2;
		const y = cfg.padding + layerIndex * (cfg.nodeHeight + cfg.verticalSpacing);

		for (let i = 0; i < layer.length; i++) {
			const id = layer[i];
			const x = startX + i * (cfg.nodeWidth + cfg.horizontalSpacing);

			layoutNodes.set(id, {
				id,
				x,
				y,
				width: cfg.nodeWidth,
				height: cfg.nodeHeight,
				layer: layerIndex
			});
		}
	}

	// Create edge paths
	const layoutEdges: LayoutEdge[] = edges.map((edge) => {
		const fromNode = layoutNodes.get(edge.from);
		const toNode = layoutNodes.get(edge.to);

		if (!fromNode || !toNode) {
			return { from: edge.from, to: edge.to, points: [] };
		}

		// Edge goes from bottom center of "from" to top center of "to"
		const fromX = fromNode.x + fromNode.width / 2;
		const fromY = fromNode.y + fromNode.height;
		const toX = toNode.x + toNode.width / 2;
		const toY = toNode.y;

		return {
			from: edge.from,
			to: edge.to,
			points: [
				{ x: fromX, y: fromY },
				{ x: toX, y: toY }
			]
		};
	});

	// Calculate total dimensions
	const totalWidth = maxWidth + 2 * cfg.padding;
	const totalHeight =
		cfg.padding * 2 + layers.length * (cfg.nodeHeight + cfg.verticalSpacing) - cfg.verticalSpacing;

	return {
		nodes: layoutNodes,
		edges: layoutEdges,
		width: Math.max(totalWidth, 200),
		height: Math.max(totalHeight, 100)
	};
}

/**
 * Generate SVG path data for an edge with curved connections.
 */
export function getEdgePath(edge: LayoutEdge): string {
	if (edge.points.length < 2) {
		return '';
	}

	const [start, end] = edge.points;
	const midY = (start.y + end.y) / 2;

	// Use a cubic bezier curve for smooth connections
	return `M ${start.x} ${start.y} C ${start.x} ${midY}, ${end.x} ${midY}, ${end.x} ${end.y}`;
}
