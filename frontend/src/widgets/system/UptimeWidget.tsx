import { useState, useEffect } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { Clock, Package } from "lucide-react";
import type { Stats } from "@/types";

function StatRow({ icon: Icon, label, value, iconClass }: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  iconClass?: string;
}) {
  return (
    <div className="flex items-center gap-3">
      <Icon className={`h-5 w-5 ${iconClass ?? "text-primary"}`} />
      <div>
        <p className="text-xs text-muted-fg">{label}</p>
        <p className="text-lg font-mono font-bold">{value}</p>
      </div>
    </div>
  );
}

export function UptimeWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full">
      <CardContent className="pt-6 space-y-4">
        <StatRow icon={Clock} label="Uptime" value={stats.UptimeStr} iconClass="text-primary" />
        <StatRow icon={Package} label="Packages" value={stats.System.Packages.toLocaleString()} iconClass="text-primary" />
      </CardContent>
    </Card>
  );
}
