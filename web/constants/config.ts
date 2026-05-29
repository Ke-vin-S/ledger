export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/v1";
export const GRAPHQL_URL = process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql";
export const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";

export const CURRENCIES = [
  { code: "LKR", label: "LKR — Sri Lanka Rupee" },
  { code: "USD", label: "USD — US Dollar" },
  { code: "EUR", label: "EUR — Euro" },
  { code: "GBP", label: "GBP — British Pound" },
  { code: "INR", label: "INR — Indian Rupee" },
  { code: "SGD", label: "SGD — Singapore Dollar" },
  { code: "AUD", label: "AUD — Australian Dollar" },
] as const;

export const CURRENCY_CODES = ["LKR", "USD", "EUR", "GBP", "INR", "SGD", "AUD"] as const;

export const SPLIT_METHODS = [
  { value: "equal", label: "Equal" },
  { value: "exact", label: "Exact amounts" },
  { value: "percentage", label: "Percentage" },
  { value: "shares", label: "Shares / weights" },
] as const;

export const SETTLEMENT_METHODS = [
  { value: "cash", label: "Cash" },
  { value: "bank_transfer", label: "Bank transfer" },
  { value: "upi", label: "UPI" },
  { value: "card", label: "Card" },
  { value: "other", label: "Other" },
] as const;

export const NOTIFICATION_POLL_INTERVAL = 30_000;

export const SELECT_CLASS =
  "w-full h-9 rounded-md border border-[hsl(var(--input))] bg-[hsl(var(--background))] px-3 text-sm focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]";

export const LOAN_STATUS_BADGE: Record<
  string,
  { label: string; variant: "default" | "secondary" | "outline" | "destructive" }
> = {
  outstanding: { label: "Outstanding", variant: "outline" },
  partially_repaid: { label: "Partially repaid", variant: "secondary" },
  settled: { label: "Settled", variant: "default" },
  disputed: { label: "Disputed", variant: "destructive" },
};

export const LOAN_STATUS_BADGE_SHORT: Record<
  string,
  { label: string; variant: "default" | "secondary" | "outline" | "destructive" }
> = {
  outstanding: { label: "Outstanding", variant: "outline" },
  partially_repaid: { label: "Partial", variant: "secondary" },
  settled: { label: "Settled", variant: "default" },
  disputed: { label: "Disputed", variant: "destructive" },
};

export const PAGE_TITLES: Record<string, string> = {
  "/dashboard": "Dashboard",
  "/loans": "Loans & Balances",
  "/notifications": "Notifications",
  "/settings": "Settings",
  "/teams": "Teams",
};

export const SETTLEMENT_STATUS_COLORS: Record<string, string> = {
  pending_confirmation:
    "text-[hsl(var(--warning,40_96%_40%))] bg-amber-50 dark:bg-amber-950/30 border-amber-200",
  confirmed: "text-[hsl(var(--primary))] bg-blue-50 dark:bg-blue-950/30 border-blue-200",
  disputed: "text-[hsl(var(--destructive))] bg-red-50 dark:bg-red-950/30 border-red-200",
};
