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
      className={cn(
        signed && isPositive && "text-[hsl(var(--positive))]",
        signed && isNegative && "text-[hsl(var(--negative))]",
        className,
      )}
    >
      {signed && isPositive && "+"}
      {formatAmount(Math.abs(amount), currency)}
    </span>
  );
}
