import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useOverview } from "@/hooks/use-overview";
import type { ServiceInfo } from "@/types";

const FALLBACK_ICON = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='currentColor' stroke-width='2'%3E%3Crect x='2' y='2' width='20' height='8' rx='2'/%3E%3Crect x='2' y='14' width='20' height='8' rx='2'/%3E%3C/svg%3E";

function StatusDotSVG({ color }: { color: string }) {
  return (
    <svg
      width="10"
      height="10"
      viewBox="0 0 10 10"
      className="absolute -bottom-0.5 -right-0.5"
    >
      <circle cx="5" cy="5" r="5" fill={color} stroke="var(--color-bg)" strokeWidth="1.5" />
    </svg>
  );
}

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

        {/* Square icon grid */}
        <div className="grid grid-cols-4 sm:grid-cols-6 md:grid-cols-8 gap-2">
          {visible.map((s) => {
            const isActive = s.active_state === "active";
            const isFailed = s.active_state === "failed" || s.failure_count > 0;
            const dotColor = isFailed
              ? "var(--color-danger)"
              : isActive
                ? "var(--color-success)"
                : "var(--color-muted-fg)";

            const card = (
              <div
                className={`
                  flex flex-col items-stretch aspect-square rounded-xl p-2 transition-all
                  ${isActive ? "bg-accent/30 hover:bg-accent/50" : "bg-muted/10"}
                  ${isFailed ? "ring-1 ring-danger/30" : ""}
                  ${!isActive ? "opacity-45 grayscale" : ""}
                `}
              >
                {/* Name above icon */}
                <span className="text-[10px] font-medium text-center leading-tight truncate mb-1">
                  {s.name}
                </span>
                {/* Square icon with status dot overlay */}
                <div className="flex-1 flex items-center justify-center relative min-h-0">
                  <div className="relative inline-flex">
                    <img
                      src={s.icon_url || FALLBACK_ICON}
                      alt=""
                      className="size-6 object-contain"
                      loading="lazy"
                      onError={(e) => { (e.target as HTMLImageElement).src = FALLBACK_ICON; }}
                    />
                    <StatusDotSVG color={dotColor} />
                  </div>
                </div>
              </div>
            );

            const tooltip = (
              <TooltipContent>
                {s.description || s.name}
                {!isActive && (
                  <span className="block text-[10px] text-muted-fg mt-0.5">
                    {s.blocked_reason || s.active_state}
                  </span>
                )}
              </TooltipContent>
            );

            if (s.homepage_url) {
              return (
                <Tooltip key={s.unit_name}>
                  <TooltipTrigger>
                    <a href={s.homepage_url} target="_blank" rel="noopener noreferrer" className="no-underline block">
                      {card}
                    </a>
                  </TooltipTrigger>
                  {tooltip}
                </Tooltip>
              );
            }

            return (
              <Tooltip key={s.unit_name}>
                <TooltipTrigger>
                  <div className="block">{card}</div>
                </TooltipTrigger>
                {tooltip}
              </Tooltip>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
