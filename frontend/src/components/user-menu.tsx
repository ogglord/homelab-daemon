import { useState, useEffect, useCallback } from "react";
import {
  Server,
  Cpu,
  Clock,
  Terminal,
  Sun,
  Moon,
  Monitor,
  Bug,
} from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import {
  Menu,
  MenuContent,
  MenuHeader,
  MenuItem,
  MenuSection,
  MenuSeparator,
  MenuTrigger,
} from "@/components/ui/menu";
import { useOverview } from "@/hooks/use-overview";

interface UserMenuProps {
  hostname?: string;
}

export function UserMenu({ hostname: propHostname }: UserMenuProps) {
  const overviewData = useOverview();
  const hostname = propHostname ?? overviewData.data?.Hostname ?? "homelab";
  const [theme, setTheme] = useState<"light" | "dark" | "system">(() => {
    try {
      const saved = localStorage.getItem("theme");
      if (saved === "light" || saved === "dark" || saved === "system") return saved;
    } catch { /* localStorage unavailable */ }
    return "system";
  });
  const overview = useOverview((o) => o?.Stats ?? null);
  const stats = overviewData.data?.Stats ?? {};

  useEffect(() => {
    const root = document.documentElement;
    if (theme === "dark") {
      root.classList.add("dark");
    } else if (theme === "light") {
      root.classList.remove("dark");
    } else {
      root.classList.toggle("dark", window.matchMedia("(prefers-color-scheme: dark)").matches);
    }
    try { localStorage.setItem("theme", theme); } catch { /* ignore */ }
  }, [theme]);

  const ThemeIcon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;

  const cycleTheme = useCallback(() => {
    setTheme((prev) => prev === "light" ? "dark" : prev === "dark" ? "system" : "light");
  }, []);

  return (
    <Menu>
      <MenuTrigger aria-label="User menu">
        <Avatar
          initials={hostname.charAt(0).toUpperCase()}
          className="size-8 cursor-pointer"
        />
      </MenuTrigger>
      <MenuContent placement="bottom end" className="min-w-56">
        <MenuSection>
          <MenuHeader>
            <span className="block font-medium">{hostname}</span>
          </MenuHeader>
          <MenuItem onAction={cycleTheme}>
            <ThemeIcon className="size-4" />
            {theme === "dark" ? "Dark" : theme === "light" ? "Light" : "System"}
          </MenuItem>
        </MenuSection>
        <MenuSeparator />
        <MenuSection label="Host">
          <MenuItem>
            <Cpu className="size-4" />
            {(stats as any).cpu_usage != null ? `${(stats as any).cpu_usage.toFixed(1)}% CPU` : "CPU —"}
          </MenuItem>
          <MenuItem>
            <Clock className="size-4" />
            {(stats as any).uptime ?? "Uptime —"}
          </MenuItem>
        </MenuSection>
        <MenuSeparator />
        <MenuSection label="Actions">
          <MenuItem onAction={() => window.open("/api/diagnostics", "_blank")}>
            <Terminal className="size-4" />
            Diagnostics
          </MenuItem>
          <MenuItem onAction={() => window.open("https://homelab.cignl.cc", "_blank")}>
            <Server className="size-4" />
            Homelab
          </MenuItem>
          <MenuItem onAction={() => window.open("/api/bug", "_blank")}>
            <Bug className="size-4 text-muted-fg" />
            Report Bug
          </MenuItem>
        </MenuSection>
      </MenuContent>
    </Menu>
  );
}
