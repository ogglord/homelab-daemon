import { useLocation } from "react-router-dom";
import { Avatar } from "@/components/ui/avatar";
import {
  Navbar,
  NavbarGap,
  NavbarItem,
  NavbarMobile,
  type NavbarProps,
  NavbarProvider,
  NavbarSection,
  NavbarSeparator,
  NavbarSpacer,
  NavbarStart,
  NavbarTrigger,
} from "@/components/ui/navbar";
import { Separator } from "@/components/ui/separator";
import { Menu, MenuContent, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { UserMenu } from "@/components/user-menu";
import type { NavItem } from "@/types";
import { ChevronDown } from "lucide-react";
import { PollIntervalSelector } from "@/components/PollIntervalSelector";

type AppNavbarProps = NavbarProps & {
  navItems: NavItem[];
  hasStreaming?: boolean;
};

export default function AppNavbar({ navItems, hasStreaming = false, ...props }: AppNavbarProps) {
  const location = useLocation();

  function isCurrent(item: NavItem): boolean {
    if (!item.path) return false;
    if (item.path === "/") return location.pathname === "/";
    return location.pathname.startsWith(item.path);
  }

  /** Navigate with a full page reload — no client-side router. */
  function hardNav(path: string, target?: string | null) {
    const isExternal = path.startsWith("http://") || path.startsWith("https://");
    if (isExternal && target === "_blank") {
      window.open(path, "_blank", "noopener,noreferrer");
    } else if (isExternal) {
      window.location.href = `/external/${encodeURIComponent(path)}`;
    } else {
      window.location.href = path;
    }
  }

  return (
    <NavbarProvider>
      <Navbar intent="float" {...props}>
        <NavbarStart>
          <a
            href="/"
            className="flex items-center gap-x-2 font-medium"
            aria-label="homelab dashboard"
          >
            <Avatar
              isSquare
              size="sm"
              className="outline-hidden"
              initials="HL"
            />
            <span>
              home<span className="text-muted-fg">lab</span>
            </span>
          </a>
        </NavbarStart>
        <NavbarGap />
        <NavbarSection>
          {navItems.map((item, idx) => {
            const hasChildren = item.children && item.children.length > 0;

            // ── Dropdown for items with children ──
            if (hasChildren) {
              return (
                <Menu key={item.label}>
                  <MenuTrigger className="inline-flex items-center gap-x-1 text-sm font-medium text-muted-fg hover:text-fg transition-colors px-2 py-1 rounded-lg">
                    {item.label}
                    <ChevronDown className="size-4" />
                  </MenuTrigger>
                  <MenuContent placement="bottom">
                    {item.path && (
                      <MenuItem href={item.path}>
                        {item.label}
                      </MenuItem>
                    )}
                    {item.path && item.children!.length > 0 && (
                      <Separator orientation="horizontal" className="my-1" />
                    )}
                    {item.children!.map((child) => {
                      const childPath = child.path ?? "";
                      const childExternal = childPath.startsWith("http://") || childPath.startsWith("https://");
                      const childBlank = childExternal && child.target === "_blank";
                      return (
                        <MenuItem
                          key={child.label}
                          href={childBlank ? childPath : childExternal ? `/external/${encodeURIComponent(childPath)}` : childPath}
                          {...(childBlank ? { target: "_blank" } : {})}
                        >
                          {child.label}
                        </MenuItem>
                      );
                    })}
                  </MenuContent>
                </Menu>
              );
            }

            // ── Single nav link ──
            if (!item.path) return null;

            const external = item.path.startsWith("http://") || item.path.startsWith("https://");
            const openBlank = external && item.target === "_blank";
            const to = external && !openBlank ? `/external/${encodeURIComponent(item.path)}` : item.path;

            // ── Pill items (Agent) ──
            if (item.pill) {
              return (
                <div key={item.path} className="flex items-center gap-x-0">
                  <NavbarItem
                    isCurrent={isCurrent(item)}
                    onPress={() => hardNav(item.path!, item.target)}
                  >
                    <span
                      className={`inline-flex items-center gap-x-1.5 rounded-full px-3 py-1.5 text-sm font-medium transition-colors ${
                        isCurrent(item)
                          ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
                          : "bg-blue-50 text-blue-600 hover:bg-blue-100 dark:bg-blue-950/20 dark:text-blue-400 dark:hover:bg-blue-900/30"
                      } ${hasStreaming ? "ring-1 ring-red-400/50" : ""}`}
                    >
                      {item.icon === "robot" && (
                        <svg
                          xmlns="http://www.w3.org/2000/svg"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          className="size-4"
                        >
                          <path d="M12 8V4H8" />
                          <rect width="16" height="12" x="4" y="8" rx="2" />
                          <path d="M2 14h2" />
                          <path d="M20 14h2" />
                          <path d="M15 13v1" />
                          <path d="M9 13v1" />
                        </svg>
                      )}
                      {item.label}
                      {hasStreaming && (
                        <span className="relative flex size-2 ml-0.5">
                          <span className="absolute inline-flex size-full animate-ping rounded-full bg-red-400 opacity-75" />
                          <span className="relative inline-flex size-2 rounded-full bg-red-500" />
                        </span>
                      )}
                    </span>
                  </NavbarItem>
                  {item.divider_after && (
                    <div className="mx-1.5 h-[18px] w-px bg-border" />
                  )}
                </div>
              );
            }

            // ── Plain nav link — full page reload ──
            return (
              <NavbarItem
                key={item.path}
                isCurrent={isCurrent(item)}
                onPress={() => hardNav(item.path!, item.target)}
              >
                {item.label}
              </NavbarItem>
            );
          })}
        </NavbarSection>
        <NavbarSpacer />
        <NavbarSection className="max-md:hidden">
          <PollIntervalSelector />
          <Separator orientation="vertical" className="mr-3 ml-1 h-5" />
          <UserMenu />
        </NavbarSection>
      </Navbar>
      <NavbarMobile>
        <NavbarTrigger />
        <NavbarSpacer />
        <NavbarSeparator className="mr-2.5" />
        <UserMenu />
      </NavbarMobile>
    </NavbarProvider>
  );
}
