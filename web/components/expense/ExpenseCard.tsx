import Link from "next/link";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Avatar } from "@/components/shared/Avatar";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type Split = {
  id: string;
  user_id: string;
  share_amount: number;
  share_units?: number;
};

type Expense = {
  id: string;
  title: string;
  amount: number;
  currency: string;
  split_method?: string;
  expense_date: string;
  is_void: boolean;
  paid_by: string;
  paid_by_name?: string;
  splits?: Split[];
  version?: number;
};

type Props = {
  expense: Expense;
  teamId?: string;
  onClick?: () => void;
};

export function ExpenseCard({ expense, teamId, onClick }: Props) {
  const content = (
    <div
      onClick={onClick}
      className={cn(
        "p-4 border rounded-xl bg-[hsl(var(--card))] shadow-sm transition-colors",
        (onClick || teamId) && "cursor-pointer hover:bg-[hsl(var(--muted))]",
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
            {expense.version && expense.version > 1 && (
              <Badge variant="outline" className="text-xs">v{expense.version}</Badge>
            )}
          </div>
          <div className="flex items-center gap-2 mt-1 flex-wrap">
            <p className="text-xs text-[hsl(var(--muted-foreground))]">
              <DateDisplay iso={expense.expense_date} />
              {expense.split_method && ` · ${expense.split_method}`}
            </p>
            {(expense.paid_by_name) && (
              <div className="flex items-center gap-1">
                <Avatar name={expense.paid_by_name} size="sm" className="h-3.5 w-3.5 text-[0.45rem]" />
                <span className="text-xs text-[hsl(var(--muted-foreground))]">{expense.paid_by_name}</span>
              </div>
            )}
          </div>
          {expense.splits && expense.splits.length > 0 && (
            <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
              {expense.splits.length} participant{expense.splits.length !== 1 ? "s" : ""}
            </p>
          )}
        </div>
        <CurrencyAmount
          amount={expense.amount}
          currency={expense.currency}
          className="text-sm font-semibold flex-shrink-0"
        />
      </div>
    </div>
  );

  if (teamId) {
    return (
      <Link href={`/teams/${teamId}/expenses/${expense.id}`}>
        {content}
      </Link>
    );
  }

  return content;
}
