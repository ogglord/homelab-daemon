import { StatusDot } from "@/components/ui/status-dot";

export function fmtBackoff(secs: number): string {
  if (secs >= 3600) return `${Math.ceil(secs / 3600)}h`;
  if (secs >= 60) return `${Math.ceil(secs / 60)}m`;
  return `${secs}s`;
}

export interface StateBadgeProps {
  state: string;
  subState?: string;
  userStopped?: boolean;
  failureCount?: number;
  backoffSeconds?: number;
  blockedReason?: string;
}

/**
 * Compact systemd-style status badge with optional retry/backoff caption.
 * User intent ("user_stopped") wins over systemd's view because services
 * that exit on SIGTERM (Plex, some podman containers) get flagged as
 * `failed` even though they stopped cleanly.
 */
export function StateBadge({
  state, subState, userStopped, failureCount = 0, backoffSeconds = 0, blockedReason = "",
}: StateBadgeProps) {
  const isActive = state === "active";
  const isFailed = state === "failed";

  let tone: "success" | "danger" | "warning" | "secondary" = "secondary";
  let label = state;

  if (isActive) {
    tone = "success";
  } else if (userStopped) {
    tone = "warning";
    label = "stopped";
  } else if (isFailed) {
    tone = "danger";
  }

  const baseLabel = subState && subState !== "dead" && subState !== "running"
    ? `${label} (${subState})`
    : label;

  return (
    <div className="flex flex-col gap-1">
      <span className="inline-flex items-center gap-1.5 text-sm capitalize">
        <StatusDot intent={tone} />
        {baseLabel}
      </span>
      {blockedReason && !isActive && (
        <span className="text-sm text-muted-fg">{blockedReason}</span>
      )}
      {!blockedReason && failureCount > 0 && !isActive && (
        <span className="text-sm text-muted-fg">
          {backoffSeconds > 0
            ? `retry in ${fmtBackoff(backoffSeconds)} (×${failureCount})`
            : `retrying… (×${failureCount})`}
        </span>
      )}
    </div>
  );
}
