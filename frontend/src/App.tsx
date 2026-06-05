import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import AppLayout from "@/components/AppLayout";

const OverviewPage  = lazy(() => import("@/pages/OverviewPage"));
const ServicesPage  = lazy(() => import("@/pages/ServicesPage"));
const VMsPage       = lazy(() => import("@/pages/VMsPage"));
const BackupsPage   = lazy(() => import("@/pages/BackupsPage"));
const DiagnosticsPage = lazy(() => import("@/pages/DiagnosticsPage"));
const IframesPage   = lazy(() => import("@/pages/IframesPage"));
const StoragePage   = lazy(() => import("@/pages/StoragePage").then(module => ({ default: module.StoragePage })));

const SecretsPage   = lazy(() => import("@/pages/SecretsPage"));
const LogsPage       = lazy(() => import("@/pages/LogsPage"));

export default function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<div className="flex items-center justify-center h-full text-muted-fg text-sm">Loading…</div>}>
        <Routes>
          <Route element={<AppLayout />}>
            <Route index element={<OverviewPage />} />
            <Route path="services" element={<ServicesPage />} />
            <Route path="vms" element={<VMsPage />} />
            <Route path="backups" element={<BackupsPage />} />
            <Route path="diagnostics" element={<DiagnosticsPage />} />
            <Route path="storage" element={<StoragePage />} />
            <Route path="secrets" element={<SecretsPage />} />
            <Route path="logs" element={<LogsPage />} />
            <Route path="external/*" element={<IframesPage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}

