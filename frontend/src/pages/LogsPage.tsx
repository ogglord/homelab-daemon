/**
 * LogsPage — browse and search service logs via VictoriaLogs.
 *
 * Uniform data model (enforced by Vector transforms):
 *   _time, level (debug|info|warn|error), module, kind, _msg
 *
 * Query mode only — 30s auto-refresh.
 */
import React, { useState, useEffect, useCallback, useRef } from "react";
import { useSearchParams } from "react-router-dom";
import { RefreshCw, Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { StatusDot } from "@/components/ui/status-dot";
import { useJsonConfig } from "@/hooks/use-json-config";
import type { LogViewerConfig, LogViewerServiceConfig, LogLine, LogFieldConfig } from "@/types/log-viewer";

// ── Level helpers ─────────────────────────────────────────────────────

const LEVEL_BADGE: Record<string, string> = {
  error: "danger", warn: "warning", info: "primary", debug: "secondary",
};

function formatLevel(level?: string): string {
  return (level ?? "").toUpperCase();
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString("en-GB", {
      hour: "2-digit", minute: "2-digit", second: "2-digit", fractionalSecondDigits: 3,
    });
  } catch { return iso; }
}

// ── Time preset helpers ───────────────────────────────────────────────

const PRESETS = [
  { label: "Last 5m",  value: "5m" },
  { label: "Last 15m", value: "15m" },
  { label: "Last 1h",  value: "1h" },
  { label: "Last 6h",  value: "6h" },
  { label: "Last 24h", value: "24h" },
];

function windowToRange(window: string): { start: string; end: string } {
  const m = window.match(/^(\d+)(m|h|d)$/);
  if (!m) return { start: "", end: "" };
  const n = parseInt(m[1], 10);
  const ms = m[2] === "m" ? n * 60_000 : m[2] === "h" ? n * 3_600_000 : n * 86_400_000;
  return {
    start: new Date(Date.now() - ms).toISOString(),
    end: new Date().toISOString(),
  };
}

// ── Single log line ───────────────────────────────────────────────────

const LogLineRow = React.memo(function LogLineRow({ line }: { line: LogLine }) {
  const time = line._time ? formatTime(line._time) : "";
  const level = formatLevel(line.level);
  const badge = (LEVEL_BADGE[line.level ?? ""] ?? "default") as any;

  return (
    <div className="flex gap-2 px-3 py-0.5 hover:bg-slate-800/50 min-h-[24px] items-baseline font-mono text-xs leading-6">
      <span className="text-slate-500 shrink-0 w-28 tabular-nums">{time}</span>
      {level && (
        <Badge intent={badge} className="shrink-0 text-center tabular-nums text-xs w-14">
          {level}
        </Badge>
      )}
      {line.module && (
        <span className="text-slate-400 shrink-0 w-24 truncate">{line.module}</span>
      )}
      <span className="text-slate-200 whitespace-pre-wrap break-all">{line._msg ?? ""}</span>
    </div>
  );
});

// ── Query hook ────────────────────────────────────────────────────────

function useLogQuery(
  config: LogViewerServiceConfig,
  fieldFilters: Record<string, string>,
  textQuery: string,
  window: string,
  maxLines: number,
) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  const fetchLogs = useCallback(() => {
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;

    const filterPairs = Object.entries(fieldFilters).filter(([, v]) => v !== "");
    let selector = config.selector;
    if (filterPairs.length > 0) {
      const filters = filterPairs.map(([k, v]) => `${k}="${v}"`).join(", ");
      const inner = selector.slice(1, -1);
      selector = `{${inner}${inner.trim() ? ", " : ""}${filters}}`;
    }
    const query = textQuery.trim() ? `${selector} ${textQuery.trim()}` : selector;

    const params = new URLSearchParams({ query, limit: String(maxLines) });
    const range = windowToRange(window);
    if (range.start) params.set("start", range.start);
    if (range.end)   params.set("end", range.end);

    setIsLoading(true);
    setError(null);

    fetch(`/api/victorialogs/select/logsql/query?${params}`, { signal: controller.signal })
      .then(r => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.text(); })
      .then(body => {
        if (controller.signal.aborted) return;
        const parsed: LogLine[] = [];
        for (const raw of body.split("\n")) {
          const t = raw.trim();
          if (!t) continue;
          try { parsed.push(JSON.parse(t) as LogLine); } catch { /* skip */ }
        }
        setLines(parsed);
        setIsLoading(false);
      })
      .catch((err: Error) => {
        if (err.name === "AbortError") return;
        if (!controller.signal.aborted) {
          setError(err.message);
          setIsLoading(false);
        }
      });
  }, [config.id, JSON.stringify(fieldFilters), textQuery, window, maxLines]);

  useEffect(() => {
    fetchLogs();
    timerRef.current = setInterval(fetchLogs, 30_000);
    return () => {
      controllerRef.current?.abort();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [fetchLogs]);

  useEffect(() => { setLines([]); }, [config.id]);

  return { lines, isLoading, error, refetch: fetchLogs };
}

// ── Outer page — loading/error gate ────────────────────────────────────

export default function LogsPage() {
  const { data: config, error: configError, loading: configLoading } =
    useJsonConfig<LogViewerConfig>("/log-viewer-config.json");

  if (configLoading) {
    return (
      <div className="flex items-center justify-center h-64 text-muted-fg">
        <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
        Loading configuration...
      </div>
    );
  }
  if (configError || !config || !config.services?.length) {
    const detail = configError
      ?? (!config ? "empty response" : `got ${config.services?.length ?? 0} services`);
    return (
      <div className="flex items-center justify-center h-64 text-red-500">
        <p>Failed to load config: {detail}</p>
      </div>
    );
  }

  return <LogsPageInner config={config} />;
}

// ── Inner page — config is guaranteed non-null, services non-empty ────

function LogsPageInner({ config }: { config: LogViewerConfig }) {
  const [searchParams, setSearchParams] = useSearchParams();

  // ─ URL state ────────────────────────────────────────────────────
  const activeServiceId = searchParams.get("service") || config.services[0].id;
  const textQuery = searchParams.get("q") || "";
  const windowPreset = searchParams.get("window") || config.defaultWindow;

  const activeService = config.services.find(s => s.id === activeServiceId)
    ?? config.services[0];

  const fieldFilters: Record<string, string> = {};
  for (const field of activeService.fields) {
    const val = searchParams.get(field.name);
    if (val) fieldFilters[field.name] = val;
  }

  // ─ Debounced search ─────────────────────────────────────────────
  const [localQuery, setLocalQuery] = useState(textQuery);
  const [debouncedQuery, setDebouncedQuery] = useState(textQuery);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    debounceRef.current = setTimeout(() => setDebouncedQuery(localQuery), 300);
    return () => clearTimeout(debounceRef.current);
  }, [localQuery]);

  const updateParams = useCallback((updates: Record<string, string | null>) => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev);
      for (const [k, v] of Object.entries(updates)) {
        if (v === null || v === "") next.delete(k);
        else next.set(k, v);
      }
      return next;
    });
  }, [setSearchParams]);

  // Set default service on first load
  const initRef = useRef(false);
  useEffect(() => {
    if (initRef.current) return;
    initRef.current = true;
    if (!searchParams.has("service")) {
      updateParams({ service: config.services[0].id });
    }
  }, []);

  // ─ Query hook ───────────────────────────────────────────────────
  const { lines, isLoading, error, refetch } = useLogQuery(
    activeService, fieldFilters, debouncedQuery, windowPreset, config.maxLines,
  );

  const handleDownload = useCallback(() => {
    if (!lines.length) return;
    const blob = new Blob([lines.map(l => JSON.stringify(l)).join("\n")], { type: "application/x-ndjson" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `logs-${activeService.id}-${Date.now()}.ndjson`;
    a.click();
    URL.revokeObjectURL(url);
  }, [lines, activeService]);

  // ─ Render ───────────────────────────────────────────────────────

  const hasFilters = Object.values(fieldFilters).some(Boolean) || localQuery.trim();

  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight text-fg">Logs</h1>
        <div className="flex items-center gap-2">
          <Button intent="outline" size="xs" onPress={refetch} isDisabled={isLoading}>
            <RefreshCw className={`w-3 h-3 mr-1 ${isLoading ? "animate-spin" : ""}`} />
            {isLoading ? "Loading..." : "Refresh"}
          </Button>
          <Button intent="outline" size="xs" onPress={handleDownload} isDisabled={!lines.length}>
            <Download className="w-3 h-3 mr-1" />
            Download
          </Button>
        </div>
      </div>

      {/* Service tabs */}
      <div className="flex gap-1 flex-wrap">
        {config.services.map(svc => (
          <Button
            key={svc.id}
            intent={activeService.id === svc.id ? "primary" : "outline"}
            size="xs"
            onPress={() => {
              const cleared: Record<string, string | null> = { service: svc.id };
              for (const f of (config.services.find(s => s.id === svc.id)?.fields ?? [])) {
                cleared[f.name] = null;
              }
              updateParams(cleared);
            }}
          >
            {svc.label}
          </Button>
        ))}
      </div>

      {/* Controls */}
      <div className="flex flex-wrap items-end gap-3">
        {activeService.fields.map(field => (
          <FieldFilter
            key={field.name}
            field={field}
            value={fieldFilters[field.name] ?? ""}
            onChange={val => updateParams({ [field.name]: val || null })}
          />
        ))}

        {hasFilters && (
          <Button
            intent="outline"
            size="xs"
            onPress={() => {
              const cleared: Record<string, string | null> = {};
              for (const f of activeService.fields) cleared[f.name] = null;
              cleared.q = null;
              updateParams(cleared);
              setLocalQuery("");
            }}
          >
            Clear filters
          </Button>
        )}

        <div className="flex-1 min-w-[180px]">
          <label className="text-xs text-muted-fg mb-1 block">Search</label>
          <Input
            placeholder="word filter..."
            value={localQuery}
            onChange={e => { setLocalQuery(e.target.value); updateParams({ q: e.target.value || null }); }}
          />
        </div>

        <FieldFilter
          field={{ name: "window", label: "Window", type: "enum", values: PRESETS.map(p => p.value) }}
          value={windowPreset}
          onChange={val => updateParams({ window: val || null })}
        />
      </div>

      {/* Status bar */}
      <div className="flex items-center gap-3 text-xs text-muted-fg">
        <StatusDot intent={isLoading ? "warning" : error ? "danger" : "success"} />
        <span>{isLoading ? "Loading..." : error ? error : `${lines.length} results`}</span>
        <span>auto-refresh: 30s</span>
      </div>

      {/* Log output */}
      <div className="bg-slate-950 rounded-lg border border-slate-800 overflow-auto font-mono text-xs leading-5 h-[60vh]">
        {lines.length === 0 && !isLoading && !error && (
          <div className="flex items-center justify-center h-full text-muted-fg italic">
            No results. Try adjusting your filters or time range.
          </div>
        )}
        {lines.length === 0 && isLoading && (
          <div className="flex items-center justify-center h-full text-muted-fg">
            <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
            Loading...
          </div>
        )}
        {lines.map((line, i) => (
          <LogLineRow key={`${line._time}-${i}`} line={line} />
        ))}
      </div>
    </div>
  );
}

// ── Field filter ───────────────────────────────────────────────────────

function FieldFilter({ field, value, onChange }: {
  field: LogFieldConfig;
  value: string;
  onChange: (val: string) => void;
}) {
  if (field.type === "enum" && field.values?.length) {
    return (
      <div>
        <label className="text-xs text-muted-fg mb-1 block">{field.label}</label>
        <select
          className="h-9 rounded-lg border border-input bg-bg px-3 py-1 text-sm text-fg outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          value={value} onChange={e => onChange(e.target.value)}
        >
          <option value="">All</option>
          {field.values.map(v => <option key={v} value={v}>{v}</option>)}
        </select>
      </div>
    );
  }
  return (
    <div>
      <label className="text-xs text-muted-fg mb-1 block">{field.label}</label>
      <Input placeholder={field.label} className="w-36" value={value}
        onChange={e => onChange(e.target.value)} />
    </div>
  );
}