import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatNumber(value: number | string | null | undefined): string {
  if (value === null || value === undefined || value === "") return "-"
  const n = typeof value === "string" ? Number(value) : value
  if (Number.isNaN(n)) return String(value)
  return new Intl.NumberFormat("zh-CN").format(n)
}
