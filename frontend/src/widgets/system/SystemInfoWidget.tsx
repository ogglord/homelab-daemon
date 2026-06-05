import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { Info } from "lucide-react";

export function SystemInfoWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  const rows = [
    ["OS", stats.System.OS],
    ["Kernel", stats.System.KernelVersion],
    ["CPU", stats.System.CPUModel],
    ...(stats.System.Motherboard ? [["Board", stats.System.Motherboard]] : []),
  ];

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Info className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">System Info</span>
        </div>

        {/* Tight key/value grid */}
        <div className="space-y-1">
          {rows.map(([key, value]) => (
            <div key={key} className="flex justify-between gap-2 text-xs">
              <span className="text-muted-fg shrink-0">{key}</span>
              <span className="font-mono text-fg truncate text-right" title={String(value)}>{value}</span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
