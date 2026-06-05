/**
 * RingGauge — SVG circular progress indicator.
 *
 * Uses stroke-dasharray/stroke-dashoffset for the arc.
 * Zero external dependencies.
 */

interface RingGaugeProps {
  /** Current value */
  value: number;
  /** Maximum value (default 100) */
  max?: number;
  /** Display label inside the ring */
  label: string;
  /** Ring stroke color */
  color?: string;
  /** Overall size in px (default 72) */
  size?: number;
  /** Stroke width (default 5) */
  strokeWidth?: number;
  className?: string;
}

export function RingGauge({
  value,
  max = 100,
  label,
  color = "var(--color-primary)",
  size = 72,
  strokeWidth = 5,
  className,
}: RingGaugeProps) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const pct = Math.min(Math.max(value / max, 0), 1);
  const offset = circumference * (1 - pct);

  return (
    <div className={className} style={{ width: size, height: size, position: "relative" }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden>
        {/* Track */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--color-border)"
          strokeWidth={strokeWidth}
        />
        {/* Fill */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
          style={{ transition: "stroke-dashoffset 0.4s ease" }}
        />
      </svg>
      {/* Center label */}
      <span
        className="absolute inset-0 flex items-center justify-center text-xs font-mono font-bold text-fg"
        style={{ lineHeight: 1 }}
      >
        {label}
      </span>
    </div>
  );
}
