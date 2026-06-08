import { ExternalLink } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Badge } from "@/components/ui/badge";
import type { PortMapping } from "@/types";

const MAX_INLINE = 3;

interface PortChipsProps {
  ports: PortMapping[];
  homepageUrl?: string;
}

export function PortChips({ ports, homepageUrl }: PortChipsProps) {
  const hasUrl = !!homepageUrl;
  const hasPorts = ports.length > 0;

  if (!hasUrl && !hasPorts) return null;

  const inline = ports.slice(0, MAX_INLINE);
  const overflow = ports.length - MAX_INLINE;

  let hostname = "";
  if (hasUrl) {
    try {
      hostname = new URL(homepageUrl!).hostname;
    } catch {
      hostname = homepageUrl!;
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-1">
      {hasUrl && (
        <a
          href={homepageUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-xs text-primary hover:underline max-w-[10rem] truncate"
          title={homepageUrl}
        >
          <ExternalLink className="size-3 shrink-0" />
          <span className="truncate">{hostname}</span>
        </a>
      )}
      {inline.map((p, i) => (
        <Badge key={i} intent="outline" isCircle={false} className="font-mono text-[10px] px-1 py-0">
          {p.host_port !== p.container_port
            ? `${p.host_port}→${p.container_port}/${p.protocol}`
            : `${p.host_port}/${p.protocol}`}
        </Badge>
      ))}
      {overflow > 0 && (
        <Tooltip>
          <TooltipTrigger>
            <Badge intent="secondary" isCircle={false} className="cursor-default text-[10px] px-1 py-0">
              +{overflow} more
            </Badge>
          </TooltipTrigger>
          <TooltipContent>
            <div className="flex flex-col gap-0.5 font-mono text-xs">
              {ports.slice(MAX_INLINE).map((p, i) => (
                <span key={i}>
                  {p.host_port !== p.container_port
                    ? `${p.host_port}→${p.container_port}/${p.protocol}`
                    : `${p.host_port}/${p.protocol}`}
                  {p.host_ip ? ` (${p.host_ip})` : ""}
                </span>
              ))}
            </div>
          </TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}
