/**
 * useJsonConfig — one-shot JSON fetch with loading/error/data state.
 *
 * Pattern: load once, guard once, then all downstream code is safe.
 * No optional chaining needed in the main render path.
 *
 * Usage:
 *   const { data, error, loading } = useJsonConfig<MyShape>("/config.json");
 *   if (loading) return <Skeleton />;
 *   if (error || !data) return <ErrorPage />;
 *   // data is guaranteed non-null from here.
 */
import { useState, useEffect } from "react";

export interface ConfigState<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
}

export function useJsonConfig<T>(url: string): ConfigState<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    fetch(url, { cache: "no-cache" })
      .then(r => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then(d => {
        if (!cancelled) { setData(d as T); setLoading(false); }
      })
      .catch((e: Error) => {
        if (!cancelled) { setError(e.message); setLoading(false); }
      });

    return () => { cancelled = true; };
  }, [url]);

  return { data, error, loading };
}
