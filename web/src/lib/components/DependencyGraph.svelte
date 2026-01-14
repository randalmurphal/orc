<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import type { DependencyGraphNode, DependencyGraphEdge } from '$lib/api';
	import { computeLayout, getEdgePath, type LayoutConfig } from '$lib/utils/graph-layout';

	interface Props {
		nodes: DependencyGraphNode[];
		edges: DependencyGraphEdge[];
		onNodeClick?: (nodeId: string) => void;
	}

	let { nodes, edges, onNodeClick }: Props = $props();

	let containerRef = $state<HTMLDivElement | null>(null);
	let svgRef = $state<SVGSVGElement | null>(null);
	let scale = $state(1);
	let translateX = $state(0);
	let translateY = $state(0);

	// Panning state
	let isPanning = $state(false);
	let panStartX = $state(0);
	let panStartY = $state(0);

	// Tooltip state
	let hoveredNode = $state<DependencyGraphNode | null>(null);
	let tooltipX = $state(0);
	let tooltipY = $state(0);

	const layoutConfig: Partial<LayoutConfig> = {
		nodeWidth: 120,
		nodeHeight: 50,
		horizontalSpacing: 50,
		verticalSpacing: 70,
		padding: 30
	};

	const layout = $derived(computeLayout(nodes, edges, layoutConfig));

	// Status colors and labels
	const statusConfig: Record<string, { color: string; label: string; fill: string }> = {
		done: { color: 'var(--status-success)', fill: 'var(--status-success-bg)', label: 'done' },
		running: { color: 'var(--accent-primary)', fill: 'var(--accent-subtle)', label: 'running' },
		blocked: { color: 'var(--status-danger)', fill: 'var(--status-danger-bg)', label: 'blocked' },
		ready: { color: 'var(--status-info)', fill: 'var(--status-info-bg)', label: 'ready' },
		pending: { color: 'var(--text-muted)', fill: 'var(--bg-tertiary)', label: 'pending' },
		paused: { color: 'var(--status-warning)', fill: 'var(--status-warning-bg)', label: 'paused' },
		failed: { color: 'var(--status-danger)', fill: 'var(--status-danger-bg)', label: 'failed' }
	};

	function getNodeConfig(status: string) {
		return statusConfig[status] || statusConfig.pending;
	}

	function handleZoomIn() {
		scale = Math.min(scale * 1.2, 3);
	}

	function handleZoomOut() {
		scale = Math.max(scale / 1.2, 0.3);
	}

	function handleFit() {
		if (!containerRef) return;
		const containerWidth = containerRef.clientWidth;
		const containerHeight = containerRef.clientHeight;

		const scaleX = containerWidth / layout.width;
		const scaleY = containerHeight / layout.height;
		scale = Math.min(scaleX, scaleY, 1) * 0.9;

		translateX = (containerWidth - layout.width * scale) / 2;
		translateY = (containerHeight - layout.height * scale) / 2;
	}

	function handleMouseDown(e: MouseEvent) {
		if (e.button === 0) {
			isPanning = true;
			panStartX = e.clientX - translateX;
			panStartY = e.clientY - translateY;
		}
	}

	function handleMouseMove(e: MouseEvent) {
		if (isPanning) {
			translateX = e.clientX - panStartX;
			translateY = e.clientY - panStartY;
		}
	}

	function handleMouseUp() {
		isPanning = false;
	}

	function handleWheel(e: WheelEvent) {
		e.preventDefault();
		const delta = e.deltaY > 0 ? 0.9 : 1.1;
		const newScale = Math.min(Math.max(scale * delta, 0.3), 3);

		// Zoom towards cursor position
		if (containerRef) {
			const rect = containerRef.getBoundingClientRect();
			const x = e.clientX - rect.left;
			const y = e.clientY - rect.top;

			translateX = x - (x - translateX) * (newScale / scale);
			translateY = y - (y - translateY) * (newScale / scale);
		}

		scale = newScale;
	}

	function handleNodeClick(nodeId: string) {
		if (onNodeClick) {
			onNodeClick(nodeId);
		} else {
			goto(`/tasks/${nodeId}`);
		}
	}

	function handleNodeMouseEnter(e: MouseEvent, node: DependencyGraphNode) {
		hoveredNode = node;
		const rect = containerRef?.getBoundingClientRect();
		if (rect) {
			tooltipX = e.clientX - rect.left;
			tooltipY = e.clientY - rect.top - 10;
		}
	}

	function handleNodeMouseLeave() {
		hoveredNode = null;
	}

	async function handleExport() {
		if (!svgRef) return;

		// Get SVG content
		const svgClone = svgRef.cloneNode(true) as SVGSVGElement;
		svgClone.setAttribute('width', String(layout.width));
		svgClone.setAttribute('height', String(layout.height));

		// Remove transform from clone
		const gElement = svgClone.querySelector('g');
		if (gElement) {
			gElement.setAttribute('transform', '');
		}

		// Convert to blob
		const svgData = new XMLSerializer().serializeToString(svgClone);
		const blob = new Blob([svgData], { type: 'image/svg+xml' });
		const url = URL.createObjectURL(blob);

		// Create canvas and render SVG
		const img = new Image();
		img.onload = () => {
			const canvas = document.createElement('canvas');
			canvas.width = layout.width;
			canvas.height = layout.height;
			const ctx = canvas.getContext('2d');
			if (ctx) {
				ctx.fillStyle = '#1a1a2e';
				ctx.fillRect(0, 0, canvas.width, canvas.height);
				ctx.drawImage(img, 0, 0);

				canvas.toBlob((blob) => {
					if (blob) {
						const downloadUrl = URL.createObjectURL(blob);
						const a = document.createElement('a');
						a.href = downloadUrl;
						a.download = 'dependency-graph.png';
						a.click();
						URL.revokeObjectURL(downloadUrl);
					}
				}, 'image/png');
			}
			URL.revokeObjectURL(url);
		};
		img.src = url;
	}

	onMount(() => {
		handleFit();
	});
</script>

<div class="dependency-graph">
	<div class="graph-toolbar">
		<div class="toolbar-group">
			<button class="toolbar-btn" onclick={handleZoomIn} title="Zoom In">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="11" cy="11" r="8"/>
					<path d="M21 21l-4.35-4.35M11 8v6M8 11h6"/>
				</svg>
			</button>
			<button class="toolbar-btn" onclick={handleZoomOut} title="Zoom Out">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="11" cy="11" r="8"/>
					<path d="M21 21l-4.35-4.35M8 11h6"/>
				</svg>
			</button>
			<button class="toolbar-btn" onclick={handleFit} title="Fit to View">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/>
				</svg>
			</button>
		</div>
		<div class="toolbar-group">
			<button class="toolbar-btn" onclick={handleExport} title="Export PNG">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 15V3"/>
				</svg>
			</button>
		</div>
	</div>

	<div
		class="graph-container"
		bind:this={containerRef}
		onmousedown={handleMouseDown}
		onmousemove={handleMouseMove}
		onmouseup={handleMouseUp}
		onmouseleave={handleMouseUp}
		onwheel={handleWheel}
		role="application"
		aria-label="Dependency graph visualization"
	>
		{#if nodes.length === 0}
			<div class="empty-state">
				<p>No tasks to display</p>
			</div>
		{:else}
			<svg
				bind:this={svgRef}
				width="100%"
				height="100%"
				class="graph-svg"
				class:panning={isPanning}
			>
				<defs>
					<marker
						id="arrowhead"
						markerWidth="10"
						markerHeight="7"
						refX="9"
						refY="3.5"
						orient="auto"
					>
						<polygon points="0 0, 10 3.5, 0 7" fill="var(--text-muted)" />
					</marker>
				</defs>

				<g transform="translate({translateX}, {translateY}) scale({scale})">
					<!-- Edges -->
					{#each layout.edges as edge}
						<path
							d={getEdgePath(edge)}
							class="edge"
							marker-end="url(#arrowhead)"
						/>
					{/each}

					<!-- Nodes -->
					{#each nodes as node}
						{@const layoutNode = layout.nodes.get(node.id)}
						{@const config = getNodeConfig(node.status)}
						{#if layoutNode}
							<g
								class="node"
								class:node-done={node.status === 'done'}
								class:node-running={node.status === 'running'}
								class:node-blocked={node.status === 'blocked'}
								transform="translate({layoutNode.x}, {layoutNode.y})"
								onclick={() => handleNodeClick(node.id)}
								onmouseenter={(e) => handleNodeMouseEnter(e, node)}
								onmouseleave={handleNodeMouseLeave}
								role="button"
								tabindex="0"
								aria-label="{node.id}: {node.title} ({node.status})"
								onkeydown={(e) => e.key === 'Enter' && handleNodeClick(node.id)}
							>
								<rect
									width={layoutNode.width}
									height={layoutNode.height}
									rx="6"
									class="node-bg"
									style="--node-color: {config.color}; --node-fill: {config.fill}"
								/>
								<text
									x={layoutNode.width / 2}
									y="18"
									class="node-id"
								>
									{node.id}
								</text>
								<text
									x={layoutNode.width / 2}
									y="36"
									class="node-status"
									style="fill: {config.color}"
								>
									({config.label})
								</text>
							</g>
						{/if}
					{/each}
				</g>
			</svg>

			<!-- Tooltip -->
			{#if hoveredNode}
				<div
					class="tooltip"
					style="left: {tooltipX}px; top: {tooltipY}px"
				>
					<div class="tooltip-title">{hoveredNode.title}</div>
					<div class="tooltip-meta">{hoveredNode.id} - {hoveredNode.status}</div>
				</div>
			{/if}
		{/if}
	</div>

	<div class="graph-legend">
		<span class="legend-title">Legend:</span>
		<span class="legend-item">
			<span class="legend-dot done"></span> done
		</span>
		<span class="legend-item">
			<span class="legend-dot ready"></span> ready
		</span>
		<span class="legend-item">
			<span class="legend-dot blocked"></span> blocked
		</span>
		<span class="legend-item">
			<span class="legend-dot running"></span> running
		</span>
	</div>
</div>

<style>
	.dependency-graph {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 400px;
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.graph-toolbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-2) var(--space-3);
		border-bottom: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
	}

	.toolbar-group {
		display: flex;
		gap: var(--space-1);
	}

	.toolbar-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		padding: 0;
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast);
	}

	.toolbar-btn:hover {
		background: var(--bg-tertiary);
		border-color: var(--border-default);
		color: var(--text-primary);
	}

	.graph-container {
		flex: 1;
		position: relative;
		overflow: hidden;
		cursor: grab;
	}

	.graph-container:active {
		cursor: grabbing;
	}

	.graph-svg {
		display: block;
	}

	.graph-svg.panning {
		cursor: grabbing;
	}

	.empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: var(--text-muted);
	}

	/* Edges */
	.edge {
		fill: none;
		stroke: var(--text-muted);
		stroke-width: 1.5;
		opacity: 0.6;
	}

	/* Nodes */
	.node {
		cursor: pointer;
		outline: none;
	}

	.node:hover .node-bg,
	.node:focus .node-bg {
		stroke-width: 2;
		filter: brightness(1.1);
	}

	.node-bg {
		fill: var(--node-fill);
		stroke: var(--node-color);
		stroke-width: 1.5;
		transition: all var(--duration-fast);
	}

	.node-running .node-bg {
		animation: pulse 2s ease-in-out infinite;
	}

	@keyframes pulse {
		0%, 100% { stroke-width: 1.5; }
		50% { stroke-width: 3; }
	}

	.node-id {
		fill: var(--text-primary);
		font-family: var(--font-mono);
		font-size: 11px;
		font-weight: 600;
		text-anchor: middle;
	}

	.node-status {
		font-size: 10px;
		text-anchor: middle;
	}

	/* Tooltip */
	.tooltip {
		position: absolute;
		padding: var(--space-2) var(--space-3);
		background: var(--bg-elevated);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-lg);
		pointer-events: none;
		transform: translateX(-50%) translateY(-100%);
		z-index: 100;
		max-width: 300px;
	}

	.tooltip-title {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.tooltip-meta {
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	/* Legend */
	.graph-legend {
		display: flex;
		align-items: center;
		gap: var(--space-4);
		padding: var(--space-2) var(--space-3);
		border-top: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.legend-title {
		font-weight: var(--font-medium);
	}

	.legend-item {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}

	.legend-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.legend-dot.done {
		background: var(--status-success);
	}

	.legend-dot.ready {
		background: var(--status-info);
	}

	.legend-dot.blocked {
		background: var(--status-danger);
	}

	.legend-dot.running {
		background: var(--accent-primary);
	}
</style>
