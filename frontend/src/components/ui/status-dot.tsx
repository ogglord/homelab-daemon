import { twMerge } from "tailwind-merge"

export type StatusDotIntent =
  | "primary"
  | "secondary"
  | "success"
  | "info"
  | "warning"
  | "danger"

const intentClass: Record<StatusDotIntent, string> = {
  primary:   "bg-primary/70",
  secondary: "bg-muted-fg/40",
  success:   "bg-success/70",
  info:      "bg-sky-500/70",
  warning:   "bg-warning/70",
  danger:    "bg-danger/70",
}

export function StatusDot({
  intent = "secondary",
  className,
  onClick,
}: {
  intent?: StatusDotIntent
  className?: string
  onClick?: React.MouseEventHandler<HTMLSpanElement>
}) {
  return (
    <span
      aria-hidden
      onClick={onClick}
      className={twMerge(
        "inline-block size-2.5 rounded-full shrink-0",
        intentClass[intent],
        className,
      )}
    />
  )
}
