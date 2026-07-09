const WIDTH = 64;
const HEIGHT = 20;

interface SparklineProps {
	values: number[];
}

/**
 * Tiny SVG trend line for percent values, scaled 0..max(100, data max).
 * Renders nothing until there are at least two samples.
 */
export function Sparkline({ values }: SparklineProps) {
	if (values.length < 2) return null;

	const max = Math.max(100, ...values);
	const stepX = WIDTH / (values.length - 1);
	const points = values
		.map((value, index) => {
			const x = index * stepX;
			const y = HEIGHT - 1 - (Math.max(value, 0) / max) * (HEIGHT - 2);
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
