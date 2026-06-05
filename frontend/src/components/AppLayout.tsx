import { useState, useEffect, useCallback } from "react";
import { Outlet } from "react-router-dom";
import { Toaster } from "@/components/ui/sonner";
import AppNavbar from "@/components/app-navbar";
import type { NavItem } from "@/types";
import { navItems as staticNav } from "@/lib/nav";

export default function AppLayout() {
  const [nav, setNav] = useState<NavItem[]>(() => {
    // Fetch the project ID on mount to build the correct sessions.cignl.cc URL.
    // This runs once and updates the Agent nav item path.
    return staticNav;
  });
  const [hasStreaming, setHasStreaming] = useState(false);

  // Fetch pi-web project UUID to link directly into the project view.
  useEffect(() => {
    fetch("/api/v1/pi/project")
      .then(r => r.json())
      .then((data: { projectId: string }) => {
        if (data.projectId) {
          setNav(prev => prev.map(item =>
            item.label === "Agent"
              ? { ...item, path: `https://sessions.cignl.cc/?project=${data.projectId}` }
              : item,
          ));
        }
      })
      .catch(() => { /* leave default link */ });
  }, []);

  // Poll pi-web sessiond health to show streaming indicator on Agent pill
  const checkStreaming = useCallback(() => {
    fetch("/api/v1/pi/streaming")
      .then(r => r.json())
      .then((data: { streaming: boolean }) => {
        setHasStreaming(data.streaming);
      })
      .catch(() => setHasStreaming(false));
  }, []);

  useEffect(() => {
    checkStreaming();
    const interval = setInterval(checkStreaming, 5000);
    return () => clearInterval(interval);
  }, [checkStreaming]);

  return (
    <div className="min-h-screen bg-bg text-fg">
      <AppNavbar navItems={nav} hasStreaming={hasStreaming} />

      <main className="mx-auto w-full max-w-screen-2xl px-4 py-6 sm:py-12">
        <Outlet />
      </main>

      <Toaster richColors theme="system" />
    </div>
  );
}
