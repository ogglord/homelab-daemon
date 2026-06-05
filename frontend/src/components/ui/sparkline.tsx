/**
 * Sparkline — lightweight inline SVG chart.
 *
 * Renders a polyline (or filled area) from an array of numbers.
 * Zero external dependencies — just SVG.
 */

interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
  filled?: boolean;
  className?: string;
  /** Min value for Y axis. Defaults to 0. */
  min?: number;
  /** Max value for Y axis. Defaults to max(data) or 100. */
  max?: number;
}

export function Sparkline({
  data,
  width = 120,
  height = 32,
  color = "var(--color-primary)",
  filled = true,
  className,
  min = 0,
  max: maxProp,
}: SparklineProps) {
  if (data.length < 2) return null;

  const max = maxProp ?? Math.max(...data, 1);
  const range = max - min || 1;
  const pad = 1; // 1px padding so strokes aren't clipped

  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * (width - pad * 2) + pad;
    const y = height - pad - ((v - min) / range) * (height - pad * 2);
    return `${x},${y}`;
  });

  const linePath = `M${points.join(" L")}`;
  const areaPath = `${linePath} L${width - pad},${height - pad} L${pad},${height - pad} Z`;

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      width={width}
      height={height}
      className={className}
      preserveAspectRatio="none"
      aria-hidden
    >
      {filled && (
        <path d={areaPath} fill={color} opacity={0.15} />
      )}
      <path
        d={linePath}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
