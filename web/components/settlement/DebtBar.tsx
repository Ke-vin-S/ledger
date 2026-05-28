import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { Avatar } from "@/components/shared/Avatar";

type Props = {
  counterpartyName: string;
  netAmount: number;
};

export function DebtBar({ counterpartyName, netAmount }: Props) {
  const owes = netAmount < 0;
  return (
    <div className="flex items-center justify-between gap-3 p-3 rounded-lg border">
      <div className="flex items-center gap-2 min-w-0">
        <Avatar name={counterpartyName} size="sm" />
        <span className="text-sm font-medium truncate">{counterpartyName}</span>
      </div>
      <div className="text-right flex-shrink-0">
        <CurrencyAmount amount={netAmount} signed className="text-sm font-semibold" />
        <p className="text-xs text-[hsl(var(--muted-foreground))]">
          {owes ? "you owe" : "owes you"}
        </p>
      </div>
    </div>
  );
}
