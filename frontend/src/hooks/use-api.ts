import { useState, useCallback } from "react";

interface UseAPIOptions {
  method?: "GET" | "POST";
  body?: unknown;
}

export function useAPI<T = unknown>() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const call = useCallback(async (url: string, options?: UseAPIOptions): Promise<T> => {
    setLoading(true);
    setError(null);
    try {
      const resp = await fetch(url, {
        method: options?.method ?? "GET",
        headers: { "Content-Type": "application/json" },
        body: options?.body ? JSON.stringify(options.body) : undefined,
      });
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: resp.statusText }));
        throw new Error(err.error ?? resp.statusText);
      }
      return resp.json();
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Unknown error";
      setError(msg);
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  return { call, loading, error };
}
