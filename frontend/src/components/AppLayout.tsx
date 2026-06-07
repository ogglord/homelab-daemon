import { Outlet } from "react-router-dom";
import { Toaster } from "@/components/ui/sonner";
import AppNavbar from "@/components/app-navbar";
import { navItems } from "@/lib/nav";

export default function AppLayout() {
  return (
    <div className="min-h-screen bg-bg text-fg">
      <AppNavbar navItems={navItems} />

      <main className="mx-auto w-full max-w-screen-2xl px-4 py-6 sm:py-12">
        <Outlet />
      </main>

      <Toaster richColors theme="system" />
    </div>
  );
}
