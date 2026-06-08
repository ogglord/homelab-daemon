import { ExternalLink, ScrollText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { RowActions, useRowContextMenu } from "@/components/data-table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { StatusDot } from "@/components/ui/status-dot";
import { StateBadge } from "./StateBadge";
import {
  renderMenuItems,
  type ServiceActionItem,
  type ServiceActionsCallbacks,
} from "./ServiceActionsMenu";

export interface ServiceCardRow extends ServiceActionItem {
  state: string;
  subState: string;
  user_stopped: boolean;
  failure_count: number;
  backoff_seconds: number;
  blocked_reason: string;
  icon_url?: string;
  homepage_url?: string;
  update_available?: boolean;
  remote_version?: string;
}

interface ServiceCardProps {
  item: ServiceCardRow;
  callbacks: ServiceActionsCallbacks;
  onTriggerUpdate: (unit: string) => void;
}

export function ServiceCard({ item, callbacks, onTriggerUpdate }: ServiceCardProps) {
  const isRunning = item.state === "active" || item.state === "activating";

  // Each card needs access to the context-menu hook, so we expose
  // onContextMenu as a prop computed by the parent's useRowContextMenu.
  const onRowCtx = useRowContextMenu<ServiceCardRow>();

  return (
    <div
      className="flex flex-col gap-2 rounded-lg border border-border bg-card p-3 w-48 min-h-[auto] shadow-sm"
      onContextMenu={onRowCtx(item)}
    >
      {/* Icon + status row */}
      <div className="flex items-start justify-between">
        <div className="relative size-8 shrink-0">
          {item.icon_url ? (
            <img
              src={item.icon_url}
              alt={item.name}
              className={`size-8 rounded object-contain transition-all ${!isRunning ? "opacity-40 grayscale" : ""}`}
            />
          ) : (
            <div className={`size-8 rounded bg-muted flex items-center justify-center text-xs font-bold text-muted-fg ${!isRunning ? "opacity-40" : ""}`}>
              {item.name[0]?.toUpperCase() ?? "?"}
            </div>
          )}
        </div>
        <div className="flex items-center gap-1">
          {item.update_available && (
            <Tooltip>
              <TooltipTrigger>
                <StatusDot
                  intent="warning"
                  className="cursor-pointer"
                  onClick={() => onTriggerUpdate(item.unit_name)}
                />
              </TooltipTrigger>
              <TooltipContent>
                Update available{item.remote_version ? ` — ${item.remote_version}` : ""}
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      </div>

      {/* Name */}
      <div className="text-sm font-medium leading-tight truncate" title={item.name}>
        {item.name}
      </div>

      {/* Status badge */}
      <StateBadge
        state={item.state}
        subState={item.subState}
        userStopped={item.user_stopped}
        failureCount={item.failure_count}
        backoffSeconds={item.backoff_seconds}
        blockedReason={item.blocked_reason}
      />

      {/* Actions row */}
      <div className="flex items-center gap-1 mt-auto pt-1">
        <Tooltip>
          <TooltipTrigger>
            <Button
              intent="plain"
              size="sq-xs"
              aria-label="View logs"
              onPress={() => callbacks.onShowLogs(item.unit_name)}
            >
              <ScrollText className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Logs</TooltipContent>
        </Tooltip>

        {item.homepage_url && (
          <a
            href={item.homepage_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center size-7 rounded text-muted-fg hover:text-fg hover:bg-muted transition-colors"
            aria-label={`Open ${item.name}`}
            title={item.homepage_url}
          >
            <ExternalLink className="size-3.5" />
          </a>
        )}

        <div className="ml-auto">
          <RowActions label={`Actions for ${item.name}`}>
            {renderMenuItems(item, callbacks)}
          </RowActions>
        </div>
      </div>
    </div>
  );
}
