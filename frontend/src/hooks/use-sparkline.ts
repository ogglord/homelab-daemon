/**
 * useSparkline — accumulates a rolling history of numeric values for sparkline charts.
 *
 * Hooks into the shared SharedPoller via useOverview so it doesn't create
 * its own fetch loop. Each call keeps a local ref buffer capped at maxPoints;
 * a new value is appended on every poll tick.
 */
import { useRef, useState, useEffect } from "react";
import { useOverview, type OverviewData } from "@/hooks/use-overview";

export function useSparkline(
  selector: (data: OverviewData) => number | undefined,
  maxPoints = 30,
): number[] {
  const bufferRef = useRef<number[]>([]);
  const [points, setPoints] = useState<number[]>([]);

  // We create a derived selector that appends to the buffer and returns it.
  // useOverview re-runs our selector on every poll tick.
  const wrappedSelector = (data: OverviewData): number[] => {
    const value = selector(data);
    if (value !== undefined && Number.isFinite(value)) {
      bufferRef.current = [...bufferRef.current, value].slice(-maxPoints);
    }
    // Return a new array so React sees a reference change
    return [...bufferRef.current];
  };

  const { data } = useOverview(wrappedSelector);

  useEffect(() => {
    if (data) setPoints(data);
  }, [data]);

  return points;
}
