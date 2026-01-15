/**
 * DependencyGraph component - Interactive DAG visualization for task dependencies.
 * Features:
 * - SVG-based rendering with zoom/pan
 * - Kahn's algorithm for topological layout
 * - Click nodes to navigate to task detail
 * - Export to PNG
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { computeLayout, getEdgePath, type LayoutConfig } from '@/lib/graph-layout';
import type { DependencyGraphNode, DependencyGraphEdge } from '@/lib/api';
import './DependencyGraph.css';

interface DependencyGraphProps {
	nodes: DependencyGraphNode[];
	edges: DependencyGraphEdge[];
	onNodeClick?: (nodeId: string) => void;
}

const layoutConfig: Partial<LayoutConfig> = {
	nodeWidth: 120,
	nodeHeight: 50,
	horizontalSpacing: 50,
	verticalSpacing: 70,
	padding: 30,
};

// Status colors and labels
const statusConfig: Record<string, { color: string; fill: string; label: string }> = {
	done: { color: 'var(--status-success)', fill: 'var(--status-success-bg)', label: 'done' },
	running: { color: 'var(--accent-primary)', fill: 'var(--accent-subtle)', label: 'running' },
	blocked: { color: 'var(--status-danger)', fill: 'var(--status-danger-bg)', label: 'blocked' },
	ready: { color: 'var(--status-info)', fill: 'var(--status-info-bg)', label: 'ready' },
	pending: { color: 'var(--text-muted)', fill: 'var(--bg-tertiary)', label: 'pending' },
	paused: { color: 'var(--status-warning)', fill: 'var(--status-warning-bg)', label: 'paused' },
	failed: { color: 'var(--status-danger)', fill: 'var(--status-danger-bg)', label: 'failed' },
};

function getNodeConfig(status: string) {
	return statusConfig[status] || statusConfig.pending;
}

export function DependencyGraph({ nodes, edges, onNodeClick }: DependencyGraphProps) {
	const navigate = useNavigate();
	const containerRef = useRef<HTMLDivElement>(null);
	const svgRef = useRef<SVGSVGElement>(null);

	const [scale, setScale] = useState(1);
	const [translateX, setTranslateX] = useState(0);
	const [translateY, setTranslateY] = useState(0);

	// Panning state
	const [isPanning, setIsPanning] = useState(false);
	const panStartRef = useRef({ x: 0, y: 0 });

	// Tooltip state
	const [hoveredNode, setHoveredNode] = useState<DependencyGraphNode | null>(null);
	const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 });

	// Compute layout
	const layout = useMemo(() => computeLayout(nodes, edges, layoutConfig), [nodes, edges]);

	// Fit to view on mount
	useEffect(() => {
		handleFit();
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const handleZoomIn = useCallback(() => {
		setScale((s) => Math.min(s * 1.2, 3));
	}, []);

	const handleZoomOut = useCallback(() => {
		setScale((s) => Math.max(s / 1.2, 0.3));
	}, []);

	const handleFit = useCallback(() => {
		if (!containerRef.current) return;
		const containerWidth = containerRef.current.clientWidth;
		const containerHeight = containerRef.current.clientHeight;

		const scaleX = containerWidth / layout.width;
		const scaleY = containerHeight / layout.height;
		const newScale = Math.min(scaleX, scaleY, 1) * 0.9;

		setScale(newScale);
		setTranslateX((containerWidth - layout.width * newScale) / 2);
		setTranslateY((containerHeight - layout.height * newScale) / 2);
	}, [layout.width, layout.height]);

	const handleMouseDown = useCallback(
		(e: React.MouseEvent) => {
			if (e.button === 0) {
				setIsPanning(true);
				panStartRef.current = { x: e.clientX - translateX, y: e.clientY - translateY };
			}
		},
		[translateX, translateY]
	);

	const handleMouseMove = useCallback(
		(e: React.MouseEvent) => {
			if (isPanning) {
				setTranslateX(e.clientX - panStartRef.current.x);
				setTranslateY(e.clientY - panStartRef.current.y);
			}
		},
		[isPanning]
	);

	const handleMouseUp = useCallback(() => {
		setIsPanning(false);
	}, []);

	const handleWheel = useCallback(
		(e: React.WheelEvent) => {
			e.preventDefault();
			const delta = e.deltaY > 0 ? 0.9 : 1.1;
			const newScale = Math.min(Math.max(scale * delta, 0.3), 3);

			// Zoom towards cursor position
			if (containerRef.current) {
				const rect = containerRef.current.getBoundingClientRect();
				const x = e.clientX - rect.left;
				const y = e.clientY - rect.top;

				setTranslateX(x - (x - translateX) * (newScale / scale));
				setTranslateY(y - (y - translateY) * (newScale / scale));
			}

			setScale(newScale);
		},
		[scale, translateX, translateY]
	);

	const handleNodeClick = useCallback(
		(nodeId: string) => {
			if (onNodeClick) {
				onNodeClick(nodeId);
			} else {
				navigate(`/tasks/${nodeId}`);
			}
		},
		[onNodeClick, navigate]
	);

	const handleNodeMouseEnter = useCallback(
		(e: React.MouseEvent, node: DependencyGraphNode) => {
			setHoveredNode(node);
			const rect = containerRef.current?.getBoundingClientRect();
			if (rect) {
				setTooltipPos({ x: e.clientX - rect.left, y: e.clientY - rect.top - 10 });
			}
		},
		[]
	);

	const handleNodeMouseLeave = useCallback(() => {
		setHoveredNode(null);
	}, []);

	const handleExport = useCallback(async () => {
		if (!svgRef.current) return;

		// Get SVG content
		const svgClone = svgRef.current.cloneNode(true) as SVGSVGElement;
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
	}, [layout.width, layout.height]);

	return (
		<div className="dependency-graph">
			<div className="graph-toolbar">
				<div className="toolbar-group">
					<button className="toolbar-btn" onClick={handleZoomIn} title="Zoom In">
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
						>
							<circle cx="11" cy="11" r="8" />
							<path d="M21 21l-4.35-4.35M11 8v6M8 11h6" />
						</svg>
					</button>
					<button className="toolbar-btn" onClick={handleZoomOut} title="Zoom Out">
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
						>
							<circle cx="11" cy="11" r="8" />
							<path d="M21 21l-4.35-4.35M8 11h6" />
						</svg>
					</button>
					<button className="toolbar-btn" onClick={handleFit} title="Fit to View">
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
						>
							<path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7" />
						</svg>
					</button>
				</div>
				<div className="toolbar-group">
					<button className="toolbar-btn" onClick={handleExport} title="Export PNG">
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
						>
							<path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 15V3" />
						</svg>
					</button>
				</div>
			</div>

			<div
				ref={containerRef}
				className="graph-container"
				onMouseDown={handleMouseDown}
				onMouseMove={handleMouseMove}
				onMouseUp={handleMouseUp}
				onMouseLeave={handleMouseUp}
				onWheel={handleWheel}
				role="application"
				aria-label="Dependency graph visualization"
			>
				{nodes.length === 0 ? (
					<div className="empty-state">
						<p>No tasks to display</p>
					</div>
				) : (
					<svg
						ref={svgRef}
						width="100%"
						height="100%"
						className={`graph-svg ${isPanning ? 'panning' : ''}`}
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

						<g transform={`translate(${translateX}, ${translateY}) scale(${scale})`}>
							{/* Edges */}
							{layout.edges.map((edge, i) => (
								<path
									key={`edge-${i}`}
									d={getEdgePath(edge)}
									className="edge"
									markerEnd="url(#arrowhead)"
								/>
							))}

							{/* Nodes */}
							{nodes.map((node) => {
								const layoutNode = layout.nodes.get(node.id);
								const config = getNodeConfig(node.status);
								if (!layoutNode) return null;

								return (
									<g
										key={node.id}
										className={`node node-${node.status}`}
										transform={`translate(${layoutNode.x}, ${layoutNode.y})`}
										onClick={() => handleNodeClick(node.id)}
										onMouseEnter={(e) => handleNodeMouseEnter(e, node)}
										onMouseLeave={handleNodeMouseLeave}
										role="button"
										tabIndex={0}
										aria-label={`${node.id}: ${node.title} (${node.status})`}
										onKeyDown={(e) =>
											e.key === 'Enter' && handleNodeClick(node.id)
										}
									>
										<rect
											width={layoutNode.width}
											height={layoutNode.height}
											rx="6"
											className="node-bg"
											style={
												{
													'--node-color': config.color,
													'--node-fill': config.fill,
												} as React.CSSProperties
											}
										/>
										<text
											x={layoutNode.width / 2}
											y="18"
											className="node-id"
										>
											{node.id}
										</text>
										<text
											x={layoutNode.width / 2}
											y="36"
											className="node-status"
											style={{ fill: config.color }}
										>
											({config.label})
										</text>
									</g>
								);
							})}
						</g>
					</svg>
				)}

				{/* Tooltip */}
				{hoveredNode && (
					<div
						className="tooltip"
						style={{ left: tooltipPos.x, top: tooltipPos.y }}
					>
						<div className="tooltip-title">{hoveredNode.title}</div>
						<div className="tooltip-meta">
							{hoveredNode.id} - {hoveredNode.status}
						</div>
					</div>
				)}
			</div>

			<div className="graph-legend">
				<span className="legend-title">Legend:</span>
				<span className="legend-item">
					<span className="legend-dot done"></span> done
				</span>
				<span className="legend-item">
					<span className="legend-dot ready"></span> ready
				</span>
				<span className="legend-item">
					<span className="legend-dot blocked"></span> blocked
				</span>
				<span className="legend-item">
					<span className="legend-dot running"></span> running
				</span>
			</div>
		</div>
	);
}
