"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTeam, useTeamMembers, useInviteMember } from "@/hooks/useTeam";
import { useExpenses, useCreateExpense, useVoidExpense } from "@/hooks/useExpenses";
import { useTeamBalances } from "@/hooks/useSettlements";
import { ActivityFeed } from "@/components/team/ActivityFeed";
import { ExpenseCard } from "@/components/expense/ExpenseCard";
import { AmountInput } from "@/components/expense/AmountInput";
import { DebtBar } from "@/components/settlement/DebtBar";
import { Skeleton } from "@/components/shared/Skeleton";
import { Avatar } from "@/components/shared/Avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Plus } from "lucide-react";
import { ApiRequestError } from "@/lib/api";

const SPLIT_METHODS = ["equal", "exact", "percentage", "shares"] as const;

const CURRENCIES = [
  "LKR", "USD", "EUR", "GBP", "INR", "SGD", "AUD",
] as const;

const expenseSchema = z.object({
  title: z.string().min(1, "Title is required"),
  amount: z.number().int().positive("Amount must be positive"),
  currency: z.string().min(1, "Currency is required"),
  split_method: z.enum(SPLIT_METHODS),
  expense_date: z.string().min(1, "Date is required"),
  note: z.string().optional(),
});
type ExpenseFormValues = z.infer<typeof expenseSchema>;

function CreateExpenseForm({ teamId, onClose }: { teamId: string; onClose: () => void }) {
  const { mutateAsync, isPending } = useCreateExpense(teamId);
  const [serverError, setServerError] = useState<string | null>(null);
  const [amount, setAmount] = useState(0);
  const [currency, setCurrency] = useState("LKR");

  const { register, handleSubmit, setValue, formState: { errors } } = useForm<ExpenseFormValues>({
    resolver: zodResolver(expenseSchema),
    defaultValues: {
      currency: "LKR",
      split_method: "equal",
      expense_date: new Date().toISOString().split("T")[0],
    },
  });

  async function onSubmit(data: ExpenseFormValues) {
    setServerError(null);
    try {
      await mutateAsync({
        title: data.title,
        amount: data.amount,
        currency: data.currency,
        split_method: data.split_method,
        expense_date: data.expense_date,
        note: data.note || undefined,
      });
      onClose();
    } catch (err) {
      if (err instanceof ApiRequestError) setServerError(err.error.message);
      else setServerError("Failed to create expense.");
    }
  }

  return (
    <Card className="mb-4">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">New expense</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {serverError && (
            <p className="text-sm text-[hsl(var(--destructive))]">{serverError}</p>
          )}

          <div className="space-y-1.5">
            <Label>Title</Label>
            <Input placeholder="e.g. Dinner at Commons" {...register("title")} />
            {errors.title && (
              <p className="text-xs text-[hsl(var(--destructive))]">{errors.title.message}</p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Currency</Label>
              <select
                className="w-full h-9 rounded-md border border-[hsl(var(--input))] bg-[hsl(var(--background))] px-3 text-sm focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]"
                {...register("currency")}
                onChange={(e) => {
                  setValue("currency", e.target.value);
                  setCurrency(e.target.value);
                }}
                defaultValue="LKR"
              >
                {CURRENCIES.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
              {errors.currency && (
                <p className="text-xs text-[hsl(var(--destructive))]">{errors.currency.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label>Date</Label>
              <Input type="date" {...register("expense_date")} />
              {errors.expense_date && (
                <p className="text-xs text-[hsl(var(--destructive))]">{errors.expense_date.message}</p>
              )}
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Amount</Label>
            <AmountInput
              value={amount}
              currency={currency}
              onChange={(v) => {
                setAmount(v);
                setValue("amount", v);
              }}
            />
            {errors.amount && (
              <p className="text-xs text-[hsl(var(--destructive))]">{errors.amount.message}</p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Split method</Label>
              <select
                className="w-full h-9 rounded-md border border-[hsl(var(--input))] bg-[hsl(var(--background))] px-3 text-sm focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]"
                {...register("split_method")}
              >
                {SPLIT_METHODS.map((m) => (
                  <option key={m} value={m}>{m.charAt(0).toUpperCase() + m.slice(1)}</option>
                ))}
              </select>
            </div>
            <div className="space-y-1.5">
              <Label>
                Note <span className="text-[hsl(var(--muted-foreground))]">(optional)</span>
              </Label>
              <Input placeholder="Any details" {...register("note")} />
            </div>
          </div>

          <div className="flex gap-2 pt-1">
            <Button type="submit" disabled={isPending} size="sm">
              {isPending ? "Adding…" : "Add expense"}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={onClose}>
              Cancel
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

export default function TeamPage() {
  const { teamId } = useParams<{ teamId: string }>();
  const { data: team, isLoading: teamLoading } = useTeam(teamId);
  const { data: members } = useTeamMembers(teamId);
  const { data: expenses, isLoading: expensesLoading } = useExpenses(teamId);
  const { data: balances } = useTeamBalances(teamId);
  const { mutate: voidExpense } = useVoidExpense(teamId);
  const { mutateAsync: inviteMember } = useInviteMember(teamId);
  const [showCreateExpense, setShowCreateExpense] = useState(false);
  const [activeTab, setActiveTab] = useState<"expenses" | "members" | "balances" | "activity">("expenses");

  if (teamLoading) {
    return (
      <div className="p-6 space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64" />
      </div>
    );
  }

  if (!team) return null;

  const tabs = [
    { key: "expenses", label: "Expenses" },
    { key: "members", label: "Members" },
    { key: "balances", label: "Balances" },
    { key: "activity", label: "Activity" },
  ] as const;

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div>
        <h1 className="text-2xl font-bold">{team.name}</h1>
        {team.description && (
          <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">{team.description}</p>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b">
        {tabs.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === key
                ? "border-[hsl(var(--primary))] text-[hsl(var(--foreground))]"
                : "border-transparent text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Expenses tab */}
      {activeTab === "expenses" && (
        <div className="space-y-4">
          {!showCreateExpense && (
            <Button size="sm" onClick={() => setShowCreateExpense(true)}>
              <Plus className="h-4 w-4 mr-1" /> Add expense
            </Button>
          )}
          {showCreateExpense && (
            <CreateExpenseForm teamId={teamId} onClose={() => setShowCreateExpense(false)} />
          )}
          {expensesLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-16" />
              ))}
            </div>
          ) : !expenses?.length ? (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">No expenses yet.</p>
          ) : (
            <div className="space-y-2">
              {expenses.map((expense) => (
                <ExpenseCard key={expense.id} expense={expense} />
              ))}
            </div>
          )}
        </div>
      )}

      {/* Members tab */}
      {activeTab === "members" && (
        <div className="space-y-3">
          {members?.map((m) => (
            <div key={m.user_id} className="flex items-center gap-3 p-3 border rounded-xl bg-[hsl(var(--card))]">
              <Avatar name={m.display_name} size="sm" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate">{m.display_name}</p>
                <p className="text-xs text-[hsl(var(--muted-foreground))]">{m.email}</p>
              </div>
              <Badge variant="secondary" className="text-xs">{m.role}</Badge>
            </div>
          ))}
          {!members?.length && (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">No members found.</p>
          )}
        </div>
      )}

      {/* Balances tab */}
      {activeTab === "balances" && (
        <div className="space-y-2">
          {balances?.length === 0 && (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">All settled up!</p>
          )}
          {balances?.map((b) => (
            <DebtBar
              key={b.counterparty_id}
              counterpartyName={b.counterparty_name}
              netAmount={b.net_amount}
            />
          ))}
        </div>
      )}

      {/* Activity tab */}
      {activeTab === "activity" && <ActivityFeed teamId={teamId} />}
    </div>
  );
}
