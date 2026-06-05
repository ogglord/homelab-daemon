"use client";

import { createContext, use, useState, useEffect, useMemo, useCallback } from "react";
import { MoreVertical } from "lucide-react";
import { CardHeader, CardTitle, CardDescription, CardAction } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Menu, MenuContent, MenuTrigger,
} from "@/components/ui/menu";
import {
  ContextMenu, ContextMenuContent,
} from "@/components/ui/context-menu";
import { SearchField, SearchInput } from "@/components/ui/search-field";

// ────────── TableContext ──────────

interface TableContextValue<T> {
  setCtxPoint: (p: { x: number; y: number } | null) => void;
  setCtxRow: (row: T | null) => void;
}

/* eslint-disable @typescript-eslint/no-explicit-any */
const TableContext = createContext<TableContextValue<any> | null>(null);

function useTableCtx<T>() {
  const ctx = use(TableContext as React.Context<TableContextValue<T> | null>);
  if (!ctx) throw new Error("useRowContextMenu must be inside <PageShell>");
  return ctx;
}

// ────────── useTable ──────────

export interface SortState {
  column: string;
  direction: "ascending" | "descending";
}

export function useTable<T>(options: {
  data: T[] | null;
  defaultSort?: SortState;
  filterField?: string;
}) {
  const [filterText, setFilterText] = useState("");
  const [sortDescriptor, setSortDescriptor] = useState<SortState>(
    options.defaultSort ?? { column: "name", direction: "ascending" },
  );

  const filtered = useMemo(() => {
    if (!options.data) return null;
    if (!filterText || !options.filterField) return options.data;
    const q = filterText.toLowerCase();
    return options.data.filter((item) =>
      String((item as Record<string, unknown>)[options.filterField!]).toLowerCase().includes(q),
    );
  }, [options.data, filterText, options.filterField]);

  const sorted = useMemo(() => {
    const list = filtered ?? [];
    return [...list].sort((a, b) => {
      const aVal = String((a as Record<string, unknown>)[sortDescriptor.column] ?? "");
      const bVal = String((b as Record<string, unknown>)[sortDescriptor.column] ?? "");
      const cmp = aVal.localeCompare(bVal, undefined, { sensitivity: "base" });
      return sortDescriptor.direction === "descending" ? -cmp : cmp;
    });
  }, [filtered, sortDescriptor]);

  const isEmpty = options.data !== null
    && (options.data.length === 0 || (filterText && filtered?.length === 0));

  return {
    filterText,
    setFilterText,
    sortDescriptor,
    setSortDescriptor,
    filtered,
    sorted,
    isEmpty,
    totalCount: options.data?.length ?? 0,
  };
}

// ────────── useRowContextMenu ──────────

/**
 * Returns a function `onRowContextMenu(item)` that produces an
 * `onContextMenu` handler for a table row. Use it inside a map:
 *
 * ```tsx
 * const onRowCtx = useRowContextMenu<MyItem>();
 * // ...
 * <TableRow onContextMenu={onRowCtx(item)}>...</TableRow>
 * ```
 */
export function useRowContextMenu<T>() {
  const { setCtxPoint, setCtxRow } = useTableCtx<T>();

  return useCallback(
    (item: T) => (e: React.MouseEvent) => {
      e.preventDefault();
      setCtxPoint({ x: e.clientX, y: e.clientY });
      setCtxRow(item);
    },
    [setCtxPoint, setCtxRow],
  );
}

// ────────── PageShell ──────────

export interface PageShellProps<T> {
  /** The data array, or null while loading. */
  data: T[] | null;
  /** Card title. */
  title: string;
  /** Card description, or a function receiving (count, filterText). */
  description?: string | ((count: number, filter: string) => string);
  /** Show a search/filter field in the card header. */
  searchable?: boolean;
  /** Controlled filter value (from useTable). */
  filter?: string;
  /** Filter change handler. */
  onFilterChange?: (v: string) => void;
  /** Search input placeholder. */
  filterPlaceholder?: string;
  /** Text shown when the table is empty. */
  emptyMessage?: string;
  /** Subtitle shown below the empty message. */
  emptyDescription?: string;
  /** Render prop for the right-click context menu. Receives the row item. */
  contextMenu?: (item: T) => React.ReactNode;
  /** Extra content rendered above the card (e.g. BulkControlBar). */
  beforeTable?: React.ReactNode;
  /** The table and any related content. */
  children?: React.ReactNode;
}

export function PageShell<T>({
  data,
  title,
  description,
  searchable,
  filter,
  onFilterChange,
  filterPlaceholder,
  emptyMessage,
  emptyDescription,
  contextMenu,
  beforeTable,
  children,
}: PageShellProps<T>) {
  const [ctxPoint, setCtxPoint] = useState<{ x: number; y: number } | null>(null);
  const [ctxRow, setCtxRow] = useState<T | null>(null);

  // Click-to-close for context menu
  useEffect(() => {
    if (!ctxPoint) return;
    const close = () => setCtxPoint(null);
    document.addEventListener("click", close);
    return () => document.removeEventListener("click", close);
  }, [ctxPoint]);

  // Loading state
  if (data === null) {
    return (
      <Skeleton isLoading>
        <div className="space-y-6">
          <div className="rounded-lg border">
            <CardHeader title={title} description="Loading…" />
            <div className="h-12" />
          </div>
        </div>
      </Skeleton>
    );
  }

  const desc = typeof description === "function"
    ? description(data.length, filter ?? "")
    : description;

  const isEmpty = data.length === 0 || (filter && data.length === 0);

  return (
    <TableContext.Provider value={{ setCtxPoint, setCtxRow }}>
      <div className="space-y-6">
        {beforeTable}

        <div className="rounded-lg border p-4">
          <CardHeader>
            <CardTitle>{title}</CardTitle>
            {desc && <CardDescription>{desc}</CardDescription>}
            {searchable && (
              <CardAction>
                <SearchField
                  aria-label={`Filter ${title}`}
                  onChange={(v) => onFilterChange?.(v)}
                  value={filter ?? ""}
                >
                  <SearchInput placeholder={filterPlaceholder ?? "Filter…"} />
                </SearchField>
              </CardAction>
            )}
          </CardHeader>

          {isEmpty ? (
            <div className="py-12 text-center">
              <p className="text-muted-fg">{emptyMessage ?? "No items found"}</p>
              {emptyDescription && (
                <p className="text-xs text-muted-fg/70 mt-1">{emptyDescription}</p>
              )}
            </div>
          ) : (
            <ContextMenu>
              {children}
              {ctxRow && contextMenu && (
                <ContextMenuContent point={ctxPoint} className="min-w-40">
                  {contextMenu(ctxRow)}
                </ContextMenuContent>
              )}
            </ContextMenu>
          )}
        </div>
      </div>
    </TableContext.Provider>
  );
}

// ────────── RowActions ──────────

export function RowActions({ label, children }: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex justify-end">
      <Menu>
        <MenuTrigger aria-label={label}>
          <MoreVertical className="size-4" />
        </MenuTrigger>
        <MenuContent placement="bottom end" className="min-w-40">
          {children}
        </MenuContent>
      </Menu>
    </div>
  );
}
