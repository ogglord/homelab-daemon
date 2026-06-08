import type { ReactNode } from "react";
import { ExternalLink } from "lucide-react";
import { Menu, MenuContent, MenuTrigger } from "@/components/ui/menu";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { StateBadge } from "./StateBadge";
import {
  renderMenuItems as renderMenuItemsHelper,
  renderContextMenuItems as renderContextMenuItemsHelper,
  type ServiceActionsCallbacks,
  type ServiceActionItem,
} from "./ServiceActionsMenu";

const FALLBACK_ICON = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='currentColor' stroke-width='2'%3E%3Crect x='2' y='2' width='20' height='8' rx='2'/%3E%3Crect x='2' y='14' width='20' height='8' rx='2'/%3E%3C/svg%3E";

// Minimal view model interface — only the fields this component needs.
interface CardItem extends ServiceActionItem {
  state: string;
  subState: string;
  user_stopped: boolean;
  failure_count: number;
  backoff_seconds: number;
  blocked_reason: string;
  daemon_enabled: boolean;
  icon_url: string;
  homepage_url: string;
}

interface ServiceCardsGridProps {
  items: CardItem[];
  buildMenuCallbacks: (afterAction?: () => void) => ServiceActionsCallbacks;
  toggleAutoStart: (unit: string, enabled: boolean) => void;
  triggerUpdate: (unit: string) => void;
  renderContextMenuItems: (item: ServiceActionItem) => ReactNode;
}

export function ServiceCardsGrid({
  items,
  buildMenuCallbacks,
  toggleAutoStart,
}: ServiceCardsGridProps) {
  const menuCallbacks = buildMenuCallbacks();

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5 gap-3 p-0.5">
      {items.map((item) => {
        const isActive = item.state === "active";
        return (
          <div
            key={item.unit_name}
            className={`
              flex flex-col rounded-xl border p-3 gap-2 transition-colors
              ${isActive ? "bg-bg hover:bg-accent/10" : "bg-muted/10 opacity-70"}
            `}
          >
            {/* Top row: icon + name + external link */}
            <div className="flex items-center gap-2 min-w-0">
              <Menu>
                <Tooltip>
                  <TooltipTrigger>
                    <MenuTrigger
                      aria-label={`Actions for ${item.name}`}
                      className="flex-shrink-0 rounded-md p-0.5 outline-hidden hover:ring-1 hover:ring-border focus-visible:ring-2 focus-visible:ring-primary transition-all"
                    >
                      <img
                        src={item.icon_url || FALLBACK_ICON}
                        alt={item.name}
                        className={`size-7 object-contain rounded ${!isActive ? "grayscale opacity-60" : ""}`}
                        loading="lazy"
                        onError={(e) => { (e.target as HTMLImageElement).src = FALLBACK_ICON; }}
                      />
                    </MenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent>Click for actions</TooltipContent>
                </Tooltip>
                <MenuContent placement="bottom start" className="min-w-40">
                  {renderMenuItemsHelper(item, menuCallbacks)}
                </MenuContent>
              </Menu>

              <span className="flex-1 min-w-0">
                <span className="text-sm font-medium truncate block">{item.name}</span>
              </span>

              {item.homepage_url && (
                <a
                  href={item.homepage_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex-shrink-0 text-muted-fg hover:text-primary transition-colors"
                  onClick={(e) => e.stopPropagation()}
                  aria-label={`Open ${item.name}`}
                >
                  <ExternalLink className="size-3.5" />
                </a>
              )}
            </div>

            {/* Status badge */}
            <StateBadge
              state={item.state}
              subState={item.subState}
              userStopped={item.user_stopped}
              failureCount={item.failure_count}
              backoffSeconds={item.backoff_seconds}
              blockedReason={item.blocked_reason}
            />

            {/* AutoStart toggle */}
            <div className="flex items-center justify-between pt-0.5">
              <span className="text-xs text-muted-fg">AutoStart</span>
              <Switch
                isSelected={item.daemon_enabled}
                onChange={(selected: boolean) => toggleAutoStart(item.unit_name, selected)}
                aria-label="AutoStart toggle"
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
