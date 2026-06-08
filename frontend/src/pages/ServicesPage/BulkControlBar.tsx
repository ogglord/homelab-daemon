import { RefreshCw, LayoutGrid, LayoutList } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface BulkControlBarProps {
  activeCount: number;
  totalCount: number;
  bulkLoading: "stop" | "start" | null;
  viewMode: "table" | "cards";
  onStartAll: () => void;
  onRequestStopAll: () => void;
  onToggleView: () => void;
}

export function BulkControlBar({
  activeCount, totalCount, bulkLoading, viewMode, onStartAll, onRequestStopAll, onToggleView,
}: BulkControlBarProps) {
  return (
    <div className="flex items-center justify-between gap-3 pb-3">
      <p className="text-sm text-muted-fg">
        {bulkLoading
          ? (bulkLoading === "stop" ? "Stopping all services…" : "Starting all services…")
          : `${activeCount} of ${totalCount} services running`}
      </p>
      <div className="flex items-center gap-2">
        <Button
          intent="plain"
          size="sq-sm"
          onPress={onToggleView}
          aria-label={viewMode === "cards" ? "Switch to table view" : "Switch to card view"}
        >
          {viewMode === "cards"
            ? <LayoutList className="size-4" />
            : <LayoutGrid className="size-4" />}
        </Button>
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
