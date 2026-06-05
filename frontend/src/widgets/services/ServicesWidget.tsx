import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusDot } from "@/components/ui/status-dot";
import { Badge } from "@/components/ui/badge";
import { useOverview } from "@/hooks/use-overview";
import type { ServiceInfo } from "@/types";
import { Server } from "lucide-react";

export function ServicesWidget() {
  const { data: services } = useOverview((o) => o?.Services);

  if (!services) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  const activeCount = services.filter((s) => s.active_state === "active").length;
  const failedServices = services.filter((s) => s.active_state === "failed" || s.failure_count > 0);

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-1.5">
            <Server className="h-3.5 w-3.5 text-success" />
            <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Services</span>
          </div>
          <Badge
            intent={activeCount === services.length ? "success" : "warning"}
            className="text-[10px] px-1.5"
          >
            {activeCount}/{services.length}
          </Badge>
        </div>

        {/* Service rows */}
        <div className="space-y-1 max-h-40 overflow-y-auto">
          {services.map((s) => {
            const isActive = s.active_state === "active";
            const isFailed = s.active_state === "failed" || s.failure_count > 0;
            return (
              <div key={s.unit_name} className="flex items-center gap-1.5 text-xs">
                <StatusDot intent={isFailed ? "danger" : isActive ? "success" : "secondary"} className="size-1.5" />
                <span className={`truncate ${isFailed ? "text-danger" : "text-fg"}`} title={s.unit_name}>
                  {s.name}
                </span>
              </div>
            );
          })}
        </div>

        {/* Failed callout */}
        {failedServices.length > 0 && (
          <div className="mt-2 pt-2 border-t border-border">
            <span className="text-[10px] font-semibold text-danger uppercase tracking-wider">
              {failedServices.length} failed
            </span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
