"use client"

import { createContext, cloneElement, use, useRef, useState } from "react"
import { twMerge } from "tailwind-merge"
import { Menu as MenuPrimitive } from "react-aria-components"
import {
  MenuContent,
  type MenuContentProps,
  MenuDescription,
  MenuHeader,
  MenuItem,
  MenuLabel,
  MenuSection,
  MenuSeparator,
  MenuShortcut,
} from "./menu"

interface ContextMenuTriggerContextType {
  buttonRef: React.RefObject<HTMLElement | null>
  contextMenuOffset: { offset: number; crossOffset: number } | null
  setContextMenuOffset: React.Dispatch<
    React.SetStateAction<{ offset: number; crossOffset: number } | null>
  >
}

const ContextMenuTriggerContext = createContext<ContextMenuTriggerContextType | undefined>(
  undefined,
)

const useContextMenuTrigger = () => {
  const context = use(ContextMenuTriggerContext)
  if (!context) {
    throw new Error("useContextMenuTrigger must be used within a ContextMenuTrigger")
  }
  return context
}

interface ContextMenuProps {
  children: React.ReactNode
}

const ContextMenu = ({ children }: ContextMenuProps) => {
  const [contextMenuOffset, setContextMenuOffset] = useState<{
    offset: number
    crossOffset: number
  } | null>(null)
  const buttonRef = useRef<HTMLElement>(null)
  return (
    <ContextMenuTriggerContext.Provider
      value={{ buttonRef, contextMenuOffset, setContextMenuOffset }}
    >
      {children}
    </ContextMenuTriggerContext.Provider>
  )
}

type ContextMenuTriggerProps = { children: React.ReactElement<any> }

const ContextMenuTrigger = ({ children }: ContextMenuTriggerProps) => {
  const { buttonRef, setContextMenuOffset } = useContextMenuTrigger()

  const onContextMenu = (e: React.MouseEvent<HTMLElement>) => {
    e.preventDefault()
    const rect = e.currentTarget.getBoundingClientRect()
    setContextMenuOffset({
      offset: e.clientY - rect.bottom,
      crossOffset: e.clientX - rect.left,
    })
  }

  return cloneElement(children, {
    ref: (el: HTMLElement | null) => { (buttonRef as React.MutableRefObject<HTMLElement | null>).current = el },
    onContextMenu: (e: React.MouseEvent<HTMLElement>) => {
      onContextMenu(e)
      if ((children.props as any).onContextMenu) (children.props as any).onContextMenu(e)
    },
    'aria-haspopup': 'menu' as const,
  } as any)
}

type ContextMenuContentProps<T> = Omit<
  MenuContentProps<T>,
  "arrow" | "isOpen" | "onOpenChange" | "triggerRef" | "placement" | "shouldFlip"
> & {
  /** Override positioning: use clientX/clientY from the event. */
  point?: { x: number; y: number } | null
}

const ContextMenuContent = <T extends object>({ point, ...props }: ContextMenuContentProps<T>) => {
  const { contextMenuOffset, setContextMenuOffset } = useContextMenuTrigger()
  const isOpen = !!(contextMenuOffset || point)
  if (!isOpen) return null

  // When point is set, render a MenuPrimitive in a fixed-position container.
  // This provides the collection context that ContextMenuItem needs.
  if (point) {
    return (
      <div
        className="fixed z-50"
        style={{ left: point.x, top: point.y }}
      >
        <div
          className="min-w-52 max-w-72 bg-overlay text-overlay-fg border border-border rounded-lg shadow-lg p-1 outline-hidden overflow-hidden"
          onClick={() => setContextMenuOffset(null)}
        >
          <MenuPrimitive
            className="outline-hidden [&_[role=menuitem]]:whitespace-nowrap"
            onAction={() => setContextMenuOffset(null)}
          >
            {props.children as React.ReactNode}
          </MenuPrimitive>
        </div>
      </div>
    )
  }

  // Normal path: positioned relative to the trigger button via popover.
  return (
    <MenuContent
      popover={{
        isOpen,
        shouldFlip: false,
        onOpenChange: () => setContextMenuOffset(null),
        placement: "bottom left",
        offset: contextMenuOffset!.offset,
        crossOffset: contextMenuOffset!.crossOffset,
      }}
      onClose={() => setContextMenuOffset(null)}
      {...props}
    />
  )
}

const ContextMenuItem = MenuItem
const ContextMenuSeparator = MenuSeparator
const ContextMenuDescription = MenuDescription
const ContextMenuSection = MenuSection
const ContextMenuHeader = MenuHeader
const ContextMenuShortcut = MenuShortcut
const ContextMenuLabel = MenuLabel

export type { ContextMenuProps }
export {
  ContextMenu,
  ContextMenuContent,
  ContextMenuDescription,
  ContextMenuHeader,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSection,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
}
