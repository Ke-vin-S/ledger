import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Formats minor units (int64 cents/paisa) to display string: "LKR 1,500.00"
export function formatAmount(minorUnits: number, currency = "LKR"): string {
  const major = minorUnits / 100;
  return `${currency} ${major.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

// Parses display string to minor units integer. Returns NaN if invalid.
export function parseAmount(display: string): number {
  const cleaned = display.replace(/[^0-9.]/g, "");
  const major = parseFloat(cleaned);
  if (isNaN(major)) return NaN;
  return Math.round(major * 100);
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
