import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { StatusDot } from "@/components/ui/status-dot";
import type { ServiceInfo } from "@/types";

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
    <Card className="h-full">
      <CardHeader title="Services Overview" />
      <CardContent className="space-y-4">
        <div className="flex items-center gap-3">
          <StatusDot intent={activeCount === services.length ? "success" : "warning"} />
          <div>
            <p className="text-xl font-mono font-bold">{activeCount} / {services.length}</p>
            <p className="text-xs text-muted-fg">Running Services</p>
          </div>
        </div>
        
        {failedServices.length > 0 && (
          <div className="mt-4 pt-4 border-t border-border">
            <p className="text-xs font-semibold text-danger mb-2">Failed Services</p>
            <div className="space-y-2 max-h-32 overflow-y-auto">
              {failedServices.map(s => (
                <div key={s.unit_name} className="flex items-center justify-between text-sm bg-danger/10 p-2 rounded">
                  <span className="text-danger truncate" title={s.name}>{s.name}</span>
                  <span className="text-xs text-danger">Failed</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
