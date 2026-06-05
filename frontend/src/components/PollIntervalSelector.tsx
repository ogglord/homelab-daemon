import { useState, useEffect } from "react";
import { getPollInterval, setPollInterval } from "@/hooks/use-overview";

const OPTIONS = [
  { value: 2000, label: "Real-time (2s)" },
  { value: 5000, label: "Fast (5s)" },
  { value: 10000, label: "Normal (10s)" },
  { value: 30000, label: "Relaxed (30s)" },
  { value: 60000, label: "Slow (60s)" },
];

export function PollIntervalSelector() {
  const [interval, setIntervalState] = useState(getPollInterval);

  // Sync when another tab changes localStorage
  useEffect(() => {
    const handler = () => setIntervalState(getPollInterval());
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  return (
    <select
      value={interval}
      onChange={(e) => {
        const ms = Number(e.target.value);
        setPollInterval(ms);
        setIntervalState(ms);
      }}
      className="text-xs text-muted-fg bg-transparent border rounded px-1 py-0.5"
      aria-label="Poll interval"
    >
      {OPTIONS.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}
