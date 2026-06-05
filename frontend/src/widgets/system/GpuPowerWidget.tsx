import { useState, useEffect } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { Sun, Cpu, Zap } from "lucide-react";
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

export function GpuPowerWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  if (!stats.Gpu?.Available) return null;

  return (
    <Card className="h-full">
      <CardContent className="pt-6 space-y-4">
        <StatRow icon={Cpu} label="Frequency" value={`${stats.Gpu.FreqMHz.toFixed(0)} MHz`} iconClass="text-primary" />
        <StatRow icon={Zap} label="GPU Power" value={`${stats.Gpu.PowerW.toFixed(1)} W`} iconClass="text-warning" />
        <StatRow icon={Sun} label="RC6 idle" value={`${stats.Gpu.RC6Pct.toFixed(0)}%`} iconClass="text-success" />
      </CardContent>
    </Card>
  );
}
