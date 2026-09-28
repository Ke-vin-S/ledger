import { cn, formatAmount } from "@/lib/utils";

type Props = {
  amount: number;
  currency?: string;
  signed?: boolean;
  className?: string;
};

export function CurrencyAmount({ amount, currency = "LKR", signed = false, className }: Props) {
  const isPositive = amount > 0;
  const isNegative = amount < 0;

  return (
    <span
      data-amount
      className={cn(
        "tabular-nums",
        signed && isPositive && "text-positive",
        signed && isNegative && "text-negative",
        className,
      )}
    >
      {signed && isPositive && "+"}
      {formatAmount(Math.abs(amount), currency)}
    </span>
  );
}
