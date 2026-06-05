"use client"

import { Search, X } from "lucide-react"
import { Button } from "react-aria-components/Button"
import type { InputProps } from "react-aria-components/Input"
import {
  SearchField as SearchFieldPrimitive,
  type SearchFieldProps,
} from "react-aria-components/SearchField"
import { twJoin } from "tailwind-merge"
import { fieldStyles } from "@/components/ui/field"
import { cx } from "@/lib/primitive"
import { Input, InputGroup } from "./input"

export function SearchField({ className, ...props }: SearchFieldProps) {
  return (
    <SearchFieldPrimitive
      data-slot="control"
      {...props}
      aria-label={props["aria-label"] ?? "Search"}
      className={cx(fieldStyles({ className: "group/search-field" }), className)}
    />
  )
}

export function SearchInput(props: InputProps) {
  return (
    <InputGroup className="[--input-gutter-end:--spacing(8)]">
      <Search data-slot="icon" className="in-disabled:opacity-50" />
      <Input {...props} />
      <Button
        className={twJoin(
          "touch-target grid place-content-center pressed:text-fg text-muted-fg hover:text-fg group-empty/search-field:invisible",
          "px-3 py-2 sm:px-2.5 sm:py-1.5 sm:text-sm/5",
        )}
      >
        <X className="size-5 sm:size-4" />
      </Button>
    </InputGroup>
  )
}
