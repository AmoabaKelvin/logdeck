const WIDTH = 64;
const HEIGHT = 20;

interface SparklineProps {
	values: number[];
}

interface Point {
	x: number;
	y: number;
}

const clampY = (y: number) => Math.min(HEIGHT - 0.5, Math.max(0.5, y));

/**
 * Catmull-Rom spline through the points, emitted as cubic beziers, so the
 * line curves smoothly instead of showing polyline kinks.
 */
function toSmoothPath(points: Point[]): string {
	let d = `M ${points[0].x.toFixed(1)},${points[0].y.toFixed(1)}`;
	for (let i = 0; i < points.length - 1; i++) {
		const p0 = points[i - 1] ?? points[i];
		const p1 = points[i];
		const p2 = points[i + 1];
		const p3 = points[i + 2] ?? p2;
		const c1x = p1.x + (p2.x - p0.x) / 6;
		const c1y = clampY(p1.y + (p2.y - p0.y) / 6);
		const c2x = p2.x - (p3.x - p1.x) / 6;
		const c2y = clampY(p2.y - (p3.y - p1.y) / 6);
		d += ` C ${c1x.toFixed(1)},${c1y.toFixed(1)} ${c2x.toFixed(1)},${c2y.toFixed(1)} ${p2.x.toFixed(1)},${p2.y.toFixed(1)}`;
	}
	return d;
}

/**
 * Tiny SVG trend line, auto-scaled to the data's own min..max range so
 * variation stays visible at any magnitude. The range floor keeps
 * near-flat data from rendering micro-noise as wild swings.
 * Renders nothing until there are at least two samples.
 */
export function Sparkline({ values }: SparklineProps) {
	if (values.length < 2) return null;

	const min = Math.min(...values);
	const range = Math.max(Math.max(...values) - min, 2);
	const stepX = WIDTH / (values.length - 1);
	const points = values.map((value, index) => ({
		x: index * stepX,
		y: HEIGHT - 1 - ((value - min) / range) * (HEIGHT - 2),
	}));

	const line = toSmoothPath(points);
	const area = `${line} L ${WIDTH},${HEIGHT} L 0,${HEIGHT} Z`;

	return (
		<svg
			width={WIDTH}
			height={HEIGHT}
			viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
			className="shrink-0 text-muted-foreground"
			aria-hidden="true"
		>
			<path d={area} fill="currentColor" className="sparkline-fill" />
			<path
				d={line}
				fill="none"
				stroke="currentColor"
				strokeWidth={1.25}
				strokeLinecap="round"
				strokeLinejoin="round"
				pathLength={1}
				className="sparkline-line"
			/>
		</svg>
	);
}
