/**
 * Single polling hook for the dashboard overview data.
 *
 * Architecture:
 *   - One shared fetcher (SharedPoller) handles all polling for the
 *     GET /api/v1/overview endpoint.
 *   - Every useOverview() call subscribes to the same poller instead of
 *     creating its own fetch loop. This eliminates the stampeding-herd
 *     problem (17 independent polls hitting the daemon simultaneously).
 *   - Stale-while-revalidate: the most recent response is returned
 *     immediately on every poll tick; the background fetch happens after
 *     the subscriber receives cached data. Perceived latency = 0ms.
 *
 * The interval is stored in localStorage (homelab-dash-poll-interval,
 * default 10s, min 2s, max 60s) and can be changed dynamically via
 * setPollInterval(). The shared poller reads it on every tick.
 */
import { useState, useEffect, useCallback } from "react";
import type { Stats, ServiceInfo, VMInfo, BackupStatus } from "@/types";

// ── Exported types matching the Go overview response ────────────────────

export interface OverviewData {
  Hostname: string;
  Stats: Stats;
  Services: ServiceInfo[];
  VMs: VMInfo[];
  Backups: BackupStatus[];
  ErrMsg?: string;
}

// ── Poll interval ───────────────────────────────────────────────────────

const POLL_KEY = "homelab-dash-poll-interval";

export function getPollInterval(): number {
  try {
    const saved = localStorage.getItem(POLL_KEY);
    if (saved) {
      const n = Number(saved);
      if (Number.isFinite(n)) return Math.max(2000, Math.min(60000, n));
    }
  } catch {
    /* localStorage unavailable */
  }
  return 10000; // default 10s
}

export function setPollInterval(ms: number) {
  const clamped = Math.max(2000, Math.min(60000, ms));
  try {
    localStorage.setItem(POLL_KEY, String(clamped));
  } catch {
    /* localStorage unavailable */
  }
}

// ── SharedPoller: one fetch, many subscribers ───────────────────────────
//
// Module-level singleton. The first subscriber starts the poll loop; the
// last subscriber stops it. Every subscriber receives the same cached data
// immediately on each tick (stale-while-revalidate), then fresh data when
// the background fetch completes.

interface PollListener {
  onData: (data: OverviewData) => void;
  onError: (err: Error, data: OverviewData | null) => void;
}

class SharedPoller {
  private subscribers = new Set<PollListener>();
  private cached: OverviewData | null = null;
  private lastError: Error | null = null;
  private timeoutId: ReturnType<typeof setTimeout> | null = null;
  private fetching = false;

  subscribe(listener: PollListener): void {
    this.subscribers.add(listener);
    // Deliver any existing cached data immediately (stale-while-revalidate
    // for late subscribers) so the UI never waits for a new fetch.
    if (this.cached !== null) {
      listener.onData(this.cached);
    }
    if (this.subscribers.size === 1) {
      // First subscriber: tick immediately so the page loads data ASAP.
      this.tick();
    }
  }

  unsubscribe(listener: PollListener): void {
    this.subscribers.delete(listener);
    if (this.subscribers.size === 0) {
      this.stop();
    }
  }



  private stop(): void {
    if (this.timeoutId !== null) {
      clearTimeout(this.timeoutId);
      this.timeoutId = null;
    }
  }

  private tick(): void {
    if (this.subscribers.size === 0) return;

    // Stale-while-revalidate: deliver cached data to all subscribers
    // immediately, then fetch fresh data in the background.
    this.deliverCached();

    // Only one in-flight fetch at a time.
    if (this.fetching) {
      return;
    }
    this.fetching = true;

    // Slow down when tab is backgrounded
    const resolvedInterval = document.hidden
      ? Math.max(getPollInterval(), 30000)
      : getPollInterval();

    fetch("/api/v1/overview")
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<OverviewData>;
      })
      .then((fresh) => {
        const prev = this.cached;

        // Detect new service failures since last poll
        if (prev?.Services && fresh.Services) {
          for (const svc of fresh.Services) {
            const old = prev.Services.find(
              (s) => s.unit_name === svc.unit_name,
            );
            if (old && svc.failure_count > old.failure_count) {
              window.dispatchEvent(
                new CustomEvent("homelab:service-failure", {
                  detail: {
                    name: svc.name,
                    failure_count: svc.failure_count,
                    backoff_seconds: svc.backoff_seconds,
                  },
                }),
              );
            }
          }
        }

        this.cached = fresh;
        this.lastError = null;
        this.fetching = false;

        // Deliver fresh data to all subscribers
        for (const listener of this.subscribers) {
          listener.onData(fresh);
        }

        if (this.subscribers.size > 0) {
          this.timeoutId = setTimeout(() => this.tick(), resolvedInterval);
        }
      })
      .catch((err: Error) => {
        this.lastError = err;
        this.fetching = false;

        for (const listener of this.subscribers) {
          listener.onError(err, this.cached);
        }

        if (this.subscribers.size > 0) {
          // Retry faster on error (2s backoff)
          this.timeoutId = setTimeout(
            () => this.tick(),
            Math.min(resolvedInterval, 2000),
          );
        }
      });
  }

  private deliverCached(): void {
    if (this.cached === null) return;
    for (const listener of this.subscribers) {
      listener.onData(this.cached);
    }
  }
}

const sharedPoller = new SharedPoller();

// ── useOverview hook ────────────────────────────────────────────────────

export function useOverview<T = OverviewData>(
  selector?: (data: OverviewData) => T,
): { data: T | null; error: Error | null; isStale: boolean } {
  const [data, setData] = useState<OverviewData | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isStale, setIsStale] = useState(true);

  useEffect(() => {
    const listener: PollListener = {
      onData(fresh: OverviewData) {
        setData(fresh);
        setError(null);
        setIsStale(false);
      },
      onError(err: Error, cached: OverviewData | null) {
        setError(err);
        // If we have cached data, the UI stays usable
        if (cached) {
          setData(cached);
          setIsStale(true);
        }
      },
    };

    sharedPoller.subscribe(listener);
    return () => sharedPoller.unsubscribe(listener);
  }, []);

  const selected = selector
    ? data
      ? selector(data)
      : null
    : (data as unknown as T);
  return { data: selected, error, isStale };
}

// ── Fallback fetcher for pages that call the daemon directly ────────────

export async function fetcher<T>(url: string): Promise<T> {
  const resp = await fetch(url);
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error ?? resp.statusText);
  }
  return resp.json() as Promise<T>;
}
