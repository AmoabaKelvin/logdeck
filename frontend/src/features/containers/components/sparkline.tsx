const WIDTH = 64;
const HEIGHT = 20;

interface SparklineProps {
	values: number[];
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
	const points = values
		.map((value, index) => {
			const x = index * stepX;
			const y = HEIGHT - 1 - ((value - min) / range) * (HEIGHT - 2);
			return `${x.toFixed(1)},${y.toFixed(1)}`;
		})
		.join(" ");

	return (
		<svg
			width={WIDTH}
			height={HEIGHT}
			viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
			className="shrink-0 text-muted-foreground"
			aria-hidden="true"
		>
			<polyline
				points={points}
				fill="none"
				stroke="currentColor"
				strokeWidth={1}
			/>
		</svg>
	);
}
