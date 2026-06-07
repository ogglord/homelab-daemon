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
};

export default function AppNavbar({ navItems, ...props }: AppNavbarProps) {
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
