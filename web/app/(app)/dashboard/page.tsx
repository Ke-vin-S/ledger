"use client";

import { useDashboardAggregates } from "@/hooks/useGraphQL";
import { useTeams } from "@/hooks/useTeam";
import { useMyExpenses, type Expense } from "@/hooks/useExpenses";
import { useMe } from "@/hooks/useAuth";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { Skeleton } from "@/components/shared/Skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import Link from "next/link";
import { ArrowRight } from "lucide-react";

function BalanceSummary() {
  const { data, isLoading } = useDashboardAggregates();
  const agg = data?.dashboardAggregates;

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-24" />
        ))}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-[hsl(var(--muted-foreground))]">
            You are owed
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CurrencyAmount
            amount={agg?.totalOwed ?? 0}
            signed
            className="text-2xl font-bold"
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-[hsl(var(--muted-foreground))]">
            You owe
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CurrencyAmount
            amount={-(agg?.totalOwing ?? 0)}
            signed
            className="text-2xl font-bold"
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-[hsl(var(--muted-foreground))]">
            Net balance
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CurrencyAmount
            amount={agg?.netBalance ?? 0}
            signed
            className="text-2xl font-bold"
          />
        </CardContent>
      </Card>
    </div>
  );
}

function TeamsList() {
  const { data: teams, isLoading } = useTeams();

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-14" />
        ))}
      </div>
    );
  }

  if (!teams?.length) {
    return (
      <p className="text-sm text-[hsl(var(--muted-foreground))] py-4">
        No teams yet.{" "}
        <Link href="/teams" className="underline">
          Create one
        </Link>
        .
      </p>
    );
  }

  return (
    <div className="space-y-2">
      {teams.map((team) => (
        <Link
          key={team.id}
          href={`/teams/${team.id}`}
          className="flex items-center justify-between p-3 rounded-lg border hover:bg-[hsl(var(--muted))] transition-colors"
        >
          <span className="font-medium text-sm">{team.name}</span>
          <ArrowRight className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
        </Link>
      ))}
    </div>
  );
}

const SCOPE_DOT: Record<string, string> = {
  team: "bg-blue-500",
  direct: "bg-violet-500",
  personal: "bg-[hsl(var(--muted-foreground))]",
};

function dateLabel(iso: string): string {
  const d = new Date(iso + "T00:00:00");
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (d.toDateString() === today.toDateString()) return "Today";
  if (d.toDateString() === yesterday.toDateString()) return "Yesterday";
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function groupExpenses(expenses: Expense[]): [string, Expense[]][] {
  const map = new Map<string, Expense[]>();
  for (const e of expenses) {
    const label = dateLabel(e.expense_date);
    if (!map.has(label)) map.set(label, []);
    map.get(label)!.push(e);
  }
  return Array.from(map.entries());
}

function TransactionRow({ expense, meId }: { expense: Expense; meId?: string }) {
  const iPaid = expense.paid_by === meId;
  const myShare = expense.splits?.find((s) => s.user_id === meId);

  const label = iPaid ? "You paid" : myShare ? "Your share" : expense.scope;
  const displayAmount = myShare && !iPaid ? -myShare.share_amount : expense.amount;
  const signed = myShare && !iPaid;

  const rowContent = (
    <div className="flex items-center gap-3 px-4 py-3 hover:bg-[hsl(var(--muted)/0.3)] transition-colors">
      <div
        className={cn(
          "h-2 w-2 rounded-full shrink-0",
          SCOPE_DOT[expense.scope] ?? "bg-[hsl(var(--muted-foreground))]",
        )}
      />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{expense.title}</p>
        <p className="text-xs text-[hsl(var(--muted-foreground))] capitalize">{label}</p>
      </div>
      <CurrencyAmount
        amount={displayAmount}
        currency={expense.currency}
        signed={signed}
        className="text-sm font-medium shrink-0"
      />
    </div>
  );

  const rowClass = "border-b last:border-b-0 border-[hsl(var(--border))]";

  if (expense.team_id) {
    return (
      <Link
        href={`/teams/${expense.team_id}/expenses/${expense.id}`}
        className={cn("block", rowClass)}
      >
        {rowContent}
      </Link>
    );
  }
  return <div className={rowClass}>{rowContent}</div>;
}

function RecentTransactions() {
  const { data: expenses, isLoading } = useMyExpenses();
  const { data: me } = useMe();

  if (isLoading) {
    return (
      <div className="rounded-lg border overflow-hidden space-y-0">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-14 rounded-none border-b last:border-b-0" />
        ))}
      </div>
    );
  }

  const recent = (expenses ?? []).filter((e) => !e.is_void).slice(0, 15);

  if (!recent.length) {
    return (
      <p className="text-sm text-[hsl(var(--muted-foreground))] py-4">
        No transactions yet.
      </p>
    );
  }

  const groups = groupExpenses(recent);

  return (
    <div className="rounded-lg border overflow-hidden">
      {groups.map(([label, items], gi) => (
        <div key={label}>
          {gi > 0 && <div className="border-t border-[hsl(var(--border))]" />}
          <div className="px-4 py-2 bg-[hsl(var(--muted)/0.5)]">
            <span className="text-xs font-semibold uppercase tracking-wide text-[hsl(var(--muted-foreground))]">
              {label}
            </span>
          </div>
          {items.map((expense) => (
            <TransactionRow key={expense.id} expense={expense} meId={me?.id} />
          ))}
        </div>
      ))}
    </div>
  );
}

export default function DashboardPage() {
  return (
    <div className="p-4 md:p-8 space-y-6 md:space-y-8 max-w-4xl">
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Balance summary</h2>
        <BalanceSummary />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Your teams</h2>
        <TeamsList />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Recent transactions</h2>
        <RecentTransactions />
      </section>
    </div>
  );
}
