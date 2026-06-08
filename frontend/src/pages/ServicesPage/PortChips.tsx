import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { PortMapping } from "@/types";

const MAX_VISIBLE = 3;

export function PortChips({ ports }: { ports: PortMapping[] }) {
  if (!ports || ports.length === 0) return null;

  const visible = ports.slice(0, MAX_VISIBLE);
  const overflow = ports.length - MAX_VISIBLE;

  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      {visible.map((p, i) => (
        <span
          key={i}
          className="inline-block rounded border border-border bg-muted/40 px-1.5 py-0.5 font-mono text-[10px] text-muted-fg leading-none"
        >
          {p.container_port}/{p.protocol}
        </span>
      ))}
      {overflow > 0 && (
        <Tooltip>
          <TooltipTrigger>
            <span className="inline-block rounded border border-border bg-muted/40 px-1.5 py-0.5 font-mono text-[10px] text-muted-fg leading-none cursor-default">
              +{overflow}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <div className="flex flex-col gap-0.5">
              {ports.slice(MAX_VISIBLE).map((p, i) => (
                <span key={i} className="font-mono text-xs">
                  {p.container_port}/{p.protocol}{p.host_ip ? ` → ${p.host_ip}:${p.host_port}` : ` → ${p.host_port}`}
                </span>
              ))}
            </div>
          </TooltipContent>
        </Tooltip>
      )}
    </span>
  );
}
