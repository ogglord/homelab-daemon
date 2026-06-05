/**
 * useSparkline — accumulates a rolling history of numeric values for sparkline charts.
 *
 * Hooks into the shared SharedPoller via useOverview so it doesn't create
 * its own fetch loop. Each call keeps a local ref buffer capped at maxPoints;
 * a new value is appended on every genuine poll tick.
 *
 * Important: appending happens inside a useEffect keyed on the raw OverviewData
 * reference, NOT inside a selector that runs on every render. Otherwise React
 * re-renders (state cascades, strict mode) would push duplicates.
 */
import { useRef, useState, useEffect } from "react";
import { useOverview, type OverviewData } from "@/hooks/use-overview";

export function useSparkline(
  selector: (data: OverviewData) => number | undefined,
  maxPoints = 30,
): number[] {
  const bufferRef = useRef<number[]>([]);
  const lastDataRef = useRef<OverviewData | null>(null);
  const [points, setPoints] = useState<number[]>([]);

  // Keep selector in a ref so the effect doesn't depend on its identity
  const selectorRef = useRef(selector);
  selectorRef.current = selector;

  // Get the full OverviewData — we compare references to detect real ticks
  const { data } = useOverview();

  useEffect(() => {
    if (!data || data === lastDataRef.current) return;
    lastDataRef.current = data;

    const value = selectorRef.current(data);
    if (value !== undefined && Number.isFinite(value)) {
      bufferRef.current = [...bufferRef.current, value].slice(-maxPoints);
      setPoints([...bufferRef.current]);
    }
  }, [data, maxPoints]);

  return points;
}
