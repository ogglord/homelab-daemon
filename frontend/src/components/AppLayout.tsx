import { Outlet } from "react-router-dom";
import { motion } from "motion/react";
import { Toaster } from "@/components/ui/sonner";
import AppNavbar from "@/components/app-navbar";
import { navItems } from "@/lib/nav";

export default function AppLayout() {
  return (
    <div className="min-h-screen text-fg">
      <AppNavbar navItems={navItems} />

      <motion.main
        className="mx-auto w-full max-w-screen-xl px-6 py-6 sm:py-12"
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25, ease: "easeOut" }}
      >
        <Outlet />
      </motion.main>

      <Toaster richColors theme="system" />
    </div>
  );
}
