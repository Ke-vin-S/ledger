import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type Expense = {
  id: string;
  title: string;
  amount: number;
  currency: string;
  split_method?: string;
  expense_date: string;
  is_void: boolean;
  paid_by: string;
};

type Props = {
  expense: Expense;
  onClick?: () => void;
};

export function ExpenseCard({ expense, onClick }: Props) {
  return (
    <div
      onClick={onClick}
      className={cn(
        "p-4 border rounded-xl bg-[hsl(var(--card))] shadow-sm",
        onClick && "cursor-pointer hover:bg-[hsl(var(--muted))] transition-colors",
        expense.is_void && "opacity-50",
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <p className="font-medium text-sm truncate">{expense.title}</p>
            {expense.is_void && (
              <Badge variant="secondary" className="text-xs">Void</Badge>
            )}
          </div>
          <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
            <DateDisplay iso={expense.expense_date} />
            {expense.split_method && ` · ${expense.split_method} split`}
          </p>
        </div>
        <CurrencyAmount
          amount={expense.amount}
          currency={expense.currency}
          className="text-sm font-semibold flex-shrink-0"
        />
      </div>
    </div>
  );
}
