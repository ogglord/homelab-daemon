import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface BulkControlBarProps {
  activeCount: number;
  totalCount: number;
  bulkLoading: "stop" | "start" | null;
  onStartAll: () => void;
  onRequestStopAll: () => void;
}

/**
 * Top-of-page row with running-count summary and Start All / Stop All
 * buttons. Stop All routes through the confirmation modal (the parent
 * owns that state).
 */
export function BulkControlBar({
  activeCount, totalCount, bulkLoading, onStartAll, onRequestStopAll,
}: BulkControlBarProps) {
  return (
    <div className="rounded-lg border p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div>
        <p className="text-sm font-medium">Service Control</p>
        <p className="text-xs text-muted-fg">
          {bulkLoading
            ? (bulkLoading === "stop" ? "Stopping all services…" : "Starting all services…")
            : `${activeCount} of ${totalCount} services running`}
        </p>
      </div>
      <div className="flex gap-2">
        <Button intent="primary" onPress={onStartAll} isDisabled={!!bulkLoading} id="btn-start-all">
          {bulkLoading === "start" ? <RefreshCw className="size-4 animate-spin" /> : null}
          Start All
        </Button>
        <Button intent="danger" onPress={onRequestStopAll} isDisabled={!!bulkLoading} id="btn-stop-all">
          {bulkLoading === "stop" ? <RefreshCw className="size-4 animate-spin" /> : null}
          Stop All
        </Button>
      </div>
    </div>
  );
}
