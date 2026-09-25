"use client";

import Link from "next/link";
import { ArrowRight, ReceiptText, UsersRound } from "lucide-react";
import { useDashboardAggregates } from "@/hooks/useGraphQL";
import { useTeams, useTeamMembers } from "@/hooks/useTeam";
import { useMyExpenses, type Expense } from "@/hooks/useExpenses";
import { useMe } from "@/hooks/useAuth";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { Avatar } from "@/components/shared/Avatar";
import { PageHeader } from "@/components/shared/PageHeader";
import { QueryBoundary } from "@/components/query-boundary";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ROUTES } from "@/constants/routes";

function Greeting() {
  const query = useMe();
  return (
    <QueryBoundary
      {...query}
      isEmpty={() => false}
      className="min-h-0 border-0 bg-transparent p-0 shadow-none"
    >
      {(me) => {
        const firstName = me.display_name.trim().split(/\s+/)[0] || "there";
        return (
          <PageHeader
            eyebrow="Your money, in one place"
            title={`Good to see you, ${firstName}.`}
            description="See what you are owed, what you owe, and where your shared money is moving."
            action={
              <Button asChild variant="outline">
                <Link href={ROUTES.teams}>
                  View teams{" "}
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </Link>
              </Button>
            }
          />
        );
      }}
    </QueryBoundary>
  );
}

function BalanceSummary() {
  const query = useDashboardAggregates();
  return (
    <QueryBoundary
      {...query}
      isEmpty={() => false}
      className="min-h-64"
      emptyMessage="No balance data yet."
    >
      {(data) => {
        const aggregates = data.dashboardAggregates;
        return (
          <section aria-labelledby="balance-heading" className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                  At a glance
                </p>
                <h2 id="balance-heading" className="mt-1 text-xl font-semibold">
                  Your balance
                </h2>
              </div>
              <ReceiptText
                className="h-5 w-5 text-primary"
                aria-hidden="true"
              />
            </div>
            <div className="grid gap-4 lg:grid-cols-[1.35fr_0.65fr]">
              <Card className="overflow-hidden border-primary/30 bg-primary text-primary-foreground shadow-sm">
                <CardContent className="flex min-h-56 flex-col justify-between gap-8 p-6 md:p-8">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-sm font-medium text-primary-foreground/75">
                        Net balance
                      </p>
                      <p className="mt-1 max-w-xs text-sm leading-6 text-primary-foreground/80">
                        {aggregates.netBalance >= 0
                          ? "You are ahead across your shared expenses."
                          : "Your shared expenses are ahead of what you are owed."}
                      </p>
                    </div>
                    <span className="rounded-full bg-primary-foreground/15 px-3 py-1 text-xs font-semibold">
                      Live
                    </span>
                  </div>
                  <CurrencyAmount
                    amount={aggregates.netBalance}
                    signed
                    className="font-display text-4xl font-semibold tracking-tight text-primary-foreground md:text-5xl"
                  />
                </CardContent>
              </Card>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
                <Card>
                  <CardContent className="flex items-center justify-between gap-4 p-5">
                    <div>
                      <p className="text-sm text-muted-foreground">
                        You are owed
                      </p>
                      <CurrencyAmount
                        amount={aggregates.totalOwed}
                        signed
                        className="mt-1 block text-2xl font-semibold"
                      />
                    </div>
                    <span className="flex size-10 items-center justify-center rounded-full bg-positive/10 text-positive">
                      <ArrowRight
                        className="h-4 w-4 -rotate-45"
                        aria-hidden="true"
                      />
                    </span>
                  </CardContent>
                </Card>
                <Card>
                  <CardContent className="flex items-center justify-between gap-4 p-5">
                    <div>
                      <p className="text-sm text-muted-foreground">You owe</p>
                      <CurrencyAmount
                        amount={-aggregates.totalOwing}
                        signed
                        className="mt-1 block text-2xl font-semibold"
                      />
                    </div>
                    <span className="flex size-10 items-center justify-center rounded-full bg-negative/10 text-negative">
                      <ArrowRight
                        className="h-4 w-4 rotate-45"
                        aria-hidden="true"
                      />
                    </span>
                  </CardContent>
                </Card>
              </div>
            </div>
          </section>
        );
      }}
    </QueryBoundary>
  );
}

function TeamCard({
  team,
}: {
  team: { id: string; name: string; description?: string; currency: string };
}) {
  const { data: members } = useTeamMembers(team.id);
  const visibleMembers = members?.slice(0, 3) ?? [];

  return (
    <Link
      href={ROUTES.team(team.id) as never}
      className="group flex min-h-36 flex-col justify-between rounded-xl border bg-card p-5 transition-colors hover:border-primary/50 hover:bg-muted/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h3 className="truncate text-base font-semibold">{team.name}</h3>
          <p className="mt-1 line-clamp-2 text-sm leading-5 text-muted-foreground">
            {team.description || "A shared space for expenses and settlements."}
          </p>
        </div>
        <ArrowRight
          className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-1"
          aria-hidden="true"
        />
      </div>
      <div className="mt-5 flex items-center justify-between gap-3">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {team.currency}
        </span>
        <div
          className="flex -space-x-2"
          aria-label={
            members?.length
              ? `${members.length} team members`
              : "Team members loading"
          }
        >
          {visibleMembers.map((member) => (
            <Avatar
              key={member.user_id}
              name={member.display_name}
              size="sm"
              className="ring-2 ring-card"
            />
          ))}
          {visibleMembers.length === 0 ? (
            <span className="flex size-7 items-center justify-center rounded-full border border-dashed bg-muted text-muted-foreground">
              <UsersRound className="h-3.5 w-3.5" aria-hidden="true" />
            </span>
          ) : null}
        </div>
      </div>
    </Link>
  );
}

function TeamsList() {
  const query = useTeams();
  return (
    <section aria-labelledby="teams-heading" className="space-y-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
            Shared spaces
          </p>
          <h2 id="teams-heading" className="mt-1 text-xl font-semibold">
            Your teams
          </h2>
        </div>
        <Button asChild variant="ghost" size="sm">
          <Link href={ROUTES.teams}>
            See all <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </Button>
      </div>
      <QueryBoundary
        {...query}
        isEmpty={(teams) => teams.length === 0}
        emptyMessage="Create a team to start sharing expenses with your people."
      >
        {(teams) => (
          <div className="grid gap-4 sm:grid-cols-2">
            {teams.map((team) => (
              <TeamCard key={team.id} team={team} />
            ))}
          </div>
        )}
      </QueryBoundary>
    </section>
  );
}

const SCOPE_DOT: Record<string, string> = {
  team: "bg-primary",
  direct: "bg-secondary-foreground",
  personal: "bg-muted-foreground",
};

function dateLabel(iso: string): string {
  const date = new Date(
    /^\d{4}-\d{2}-\d{2}$/.test(iso) ? `${iso}T00:00:00` : iso,
  );
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (date.toDateString() === today.toDateString()) return "Today";
  if (date.toDateString() === yesterday.toDateString()) return "Yesterday";
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function groupExpenses(expenses: Expense[]): [string, Expense[]][] {
  const groups = new Map<string, Expense[]>();
  for (const expense of expenses) {
    const label = dateLabel(expense.expense_date);
    const current = groups.get(label) ?? [];
    current.push(expense);
    groups.set(label, current);
  }
  return Array.from(groups.entries());
}

function TransactionRow({
  expense,
  meId,
}: {
  expense: Expense;
  meId?: string;
}) {
  const paidByMe = expense.paid_by === meId;
  const myShare = expense.splits?.find((split) => split.user_id === meId);
  const label = paidByMe ? "You paid" : myShare ? "Your share" : expense.scope;
  const displayAmount =
    myShare && !paidByMe ? -myShare.share_amount : expense.amount;
  const content = (
    <div className="flex items-center gap-3 px-4 py-4 transition-colors hover:bg-muted/30">
      <span
        className={cn(
          "h-2.5 w-2.5 shrink-0 rounded-full",
          SCOPE_DOT[expense.scope] ?? "bg-muted-foreground",
        )}
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{expense.title}</p>
        <p className="mt-0.5 text-xs capitalize text-muted-foreground">
          {label}
        </p>
      </div>
      <CurrencyAmount
        amount={displayAmount}
        currency={expense.currency}
        signed={Boolean(myShare && !paidByMe)}
        className="shrink-0 text-sm font-semibold"
      />
    </div>
  );
  const className = "block border-b border-border last:border-b-0";
  return expense.team_id ? (
    <Link
      href={`${ROUTES.team(expense.team_id)}/expenses/${expense.id}` as never}
      className={className}
    >
      {content}
    </Link>
  ) : (
    <div className={className}>{content}</div>
  );
}

function RecentTransactions() {
  const expensesQuery = useMyExpenses();
  const meQuery = useMe();

  return (
    <section aria-labelledby="activity-heading" className="space-y-4">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          Latest movement
        </p>
        <h2 id="activity-heading" className="mt-1 text-xl font-semibold">
          Recent transactions
        </h2>
      </div>
      <QueryBoundary
        {...meQuery}
        isEmpty={() => false}
        className="min-h-0 border-0 bg-transparent p-0 shadow-none"
      >
        {(me) => (
          <QueryBoundary
            {...expensesQuery}
            isEmpty={(expenses) =>
              expenses.filter((expense) => !expense.is_void).length === 0
            }
            emptyMessage="Your recent expenses will appear here."
          >
            {(expenses) => {
              const recent = expenses
                .filter((expense) => !expense.is_void)
                .slice(0, 15);
              return (
                <div className="overflow-hidden rounded-xl border bg-card">
                  {groupExpenses(recent).map(([label, items], index) => (
                    <div
                      key={label}
                      className={
                        index > 0 ? "border-t border-border" : undefined
                      }
                    >
                      <div className="bg-muted/40 px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                        {label}
                      </div>
                      {items.map((expense) => (
                        <TransactionRow
                          key={expense.id}
                          expense={expense}
                          meId={me.id}
                        />
                      ))}
                    </div>
                  ))}
                </div>
              );
            }}
          </QueryBoundary>
        )}
      </QueryBoundary>
    </section>
  );
}

export default function DashboardPage() {
  return (
    <main className="mx-auto max-w-6xl space-y-10 p-4 md:p-8 lg:space-y-12">
      <Greeting />
      <div className="grid gap-10 lg:grid-cols-[1.1fr_0.9fr] lg:items-start">
        <BalanceSummary />
        <TeamsList />
      </div>
      <RecentTransactions />
    </main>
  );
}
