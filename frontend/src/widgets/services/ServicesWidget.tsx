import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusDot } from "@/components/ui/status-dot";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useOverview } from "@/hooks/use-overview";
import type { ServiceInfo } from "@/types";

const FALLBACK_ICON = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='currentColor' stroke-width='2'%3E%3Crect x='2' y='2' width='20' height='8' rx='2'/%3E%3Crect x='2' y='14' width='20' height='8' rx='2'/%3E%3C/svg%3E";

export function ServicesWidget() {
  const { data: services } = useOverview((o) => o?.Services);

  if (!services) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  const visible = services.filter((s) => s.name !== "postgresql" && s.name !== "caddy");

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header */}
        <div className="flex items-center gap-1.5 mb-3">
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Services</span>
          <span className="text-[10px] font-mono text-muted-fg">({visible.filter(s => s.active_state === "active").length}/{visible.length} online)</span>
        </div>

        {/* Icon grid */}
        <div className="grid grid-cols-4 sm:grid-cols-6 md:grid-cols-8 gap-2">
          {visible.map((s) => {
            const isActive = s.active_state === "active";
            const isFailed = s.active_state === "failed" || s.failure_count > 0;
            const hoverable = s.active_state !== "inactive" || !!s.homepage_url;

            const inner = (
              <div
                className={`
                  flex flex-col items-center gap-1 p-2 rounded-lg transition-all
                  ${isActive ? "bg-accent/30 hover:bg-accent/50" : "bg-muted/10"}
                  ${isFailed ? "ring-1 ring-danger/30" : ""}
                  ${!isActive ? "opacity-45 grayscale" : ""}
                  ${hoverable ? "cursor-pointer" : "cursor-default"}
                `}
              >
                <img
                  src={s.icon_url || FALLBACK_ICON}
                  alt=""
                  className="size-6 object-contain"
                  loading="lazy"
                  onError={(e) => { (e.target as HTMLImageElement).src = FALLBACK_ICON; }}
                />
                <span className="text-[9px] text-center leading-tight truncate w-full">{s.name}</span>
                <StatusDot intent={isFailed ? "danger" : isActive ? "success" : "secondary"} className="size-1.5 shrink-0" />
              </div>
            );

            if (s.homepage_url) {
              return (
                <Tooltip key={s.unit_name}>
                  <TooltipTrigger>
                    <a href={s.homepage_url} target="_blank" rel="noopener noreferrer" className="no-underline">
                      {inner}
                    </a>
                  </TooltipTrigger>
                  <TooltipContent>
                    {s.description || s.name}
                    {!isActive && (
                      <span className="block text-[10px] text-muted-fg mt-0.5">
                        {s.blocked_reason || s.active_state}
                      </span>
                    )}
                  </TooltipContent>
                </Tooltip>
              );
            }

            return (
              <Tooltip key={s.unit_name}>
                <TooltipTrigger>
                  <div>{inner}</div>
                </TooltipTrigger>
                <TooltipContent>
                  {s.description || s.name}
                </TooltipContent>
              </Tooltip>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
