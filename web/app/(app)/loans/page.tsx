"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import Link from "next/link";
import { useLoans, useCreateLoan } from "@/hooks/useLoans";
import { useMyBalances } from "@/hooks/useSettlements";
import { DebtBar } from "@/components/settlement/DebtBar";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Avatar } from "@/components/shared/Avatar";
import { Skeleton } from "@/components/shared/Skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AmountInput } from "@/components/expense/AmountInput";
import { Card, CardContent } from "@/components/ui/card";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Plus, ArrowUpRight, ArrowDownLeft } from "lucide-react";
import { ApiRequestError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { CURRENCY_CODES, LOAN_STATUS_BADGE_SHORT, SELECT_CLASS } from "@/constants/config";
import { ROUTES } from "@/constants/routes";

const loanSchema = z.object({
  direction: z.enum(["lent", "borrowed"]),
  amount: z.number().int().positive("Amount must be positive"),
  currency: z.string().min(1),
  counterparty_name: z.string().min(1, "Name is required"),
  note: z.string().optional(),
  loan_date: z.string().min(1, "Date is required"),
});
type LoanFormValues = z.infer<typeof loanSchema>;

function CreateLoanForm({ onSuccess }: { onSuccess: () => void }) {
  const { mutateAsync, isPending } = useCreateLoan();
  const [amount, setAmount] = useState(0);
  const [currency, setCurrency] = useState("LKR");
  const [serverError, setServerError] = useState("");

  const { register, handleSubmit, setValue, watch, formState: { errors } } = useForm<LoanFormValues>({
    resolver: zodResolver(loanSchema),
    defaultValues: {
      direction: "lent",
      currency: "LKR",
      loan_date: new Date().toISOString().split("T")[0],
    },
  });

  const selectedDirection = watch("direction");

  async function onSubmit(data: LoanFormValues) {
    setServerError("");
    try {
      await mutateAsync({
        direction: data.direction,
        amount: data.amount,
        currency: data.currency,
        counterparty_name: data.counterparty_name,
        note: data.note || undefined,
        loan_date: data.loan_date,
      });
      onSuccess();
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.error.message : "Failed to create loan");
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
      {serverError && (
        <div className="p-3 rounded-lg bg-[hsl(var(--destructive)/0.1)] text-[hsl(var(--destructive))] text-sm">
          {serverError}
        </div>
      )}

      {/* Direction toggle */}
      <div className="space-y-1.5">
        <Label>Direction</Label>
        <div className="grid grid-cols-2 gap-2">
          {(["lent", "borrowed"] as const).map((dir) => (
            <label key={dir} className="cursor-pointer">
              <input type="radio" value={dir} {...register("direction")} className="sr-only" />
              <div className={cn(
                "flex items-center justify-center gap-2 p-2.5 rounded-lg border text-sm font-medium transition-all",
                selectedDirection === dir
                  ? "bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] border-[hsl(var(--primary))]"
                  : "bg-[hsl(var(--background))] text-[hsl(var(--foreground))] border-[hsl(var(--input))] hover:bg-[hsl(var(--muted))]",
              )}>
                {dir === "lent" ? <ArrowUpRight className="h-4 w-4" /> : <ArrowDownLeft className="h-4 w-4" />}
                {dir === "lent" ? "I lent" : "I borrowed"}
              </div>
            </label>
          ))}
        </div>
      </div>

      {/* Counterparty */}
      <div className="space-y-1.5">
        <Label>Person</Label>
        <Input placeholder="Name of the other person" {...register("counterparty_name")} />
        {errors.counterparty_name && <p className="text-xs text-[hsl(var(--destructive))]">{errors.counterparty_name.message}</p>}
      </div>

      {/* Currency + Date */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label>Currency</Label>
          <select
            className={SELECT_CLASS}
            {...register("currency")}
            onChange={(e) => { setValue("currency", e.target.value); setCurrency(e.target.value); }}
            defaultValue="LKR"
          >
            {CURRENCY_CODES.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
        </div>
        <div className="space-y-1.5">
          <Label>Date</Label>
          <Input type="date" {...register("loan_date")} />
          {errors.loan_date && <p className="text-xs text-[hsl(var(--destructive))]">{errors.loan_date.message}</p>}
        </div>
      </div>

      {/* Amount */}
      <div className="space-y-1.5">
        <Label>Amount</Label>
        <AmountInput value={amount} currency={currency} onChange={(v) => { setAmount(v); setValue("amount", v); }} />
        {errors.amount && <p className="text-xs text-[hsl(var(--destructive))]">{errors.amount.message}</p>}
      </div>

      {/* Note */}
      <div className="space-y-1.5">
        <Label>Note <span className="text-[hsl(var(--muted-foreground))]">(optional)</span></Label>
        <Input placeholder="What was this for?" {...register("note")} />
      </div>

      <div className="flex gap-3 pt-2">
        <Button type="submit" disabled={isPending} className="flex-1">
          {isPending ? "Creating…" : "Create loan"}
        </Button>
      </div>
    </form>
  );
}

type TabKey = "lent" | "borrowed" | "balances";

export default function LoansPage() {
  const [tab, setTab] = useState<TabKey>("lent");
  const [sheetOpen, setSheetOpen] = useState(false);

  const { data: lentLoans, isLoading: lentLoading } = useLoans("lent");
  const { data: borrowedLoans, isLoading: borrowedLoading } = useLoans("borrowed");
  const { data: balances, isLoading: balancesLoading } = useMyBalances();

  const tabs: { key: TabKey; label: string }[] = [
    { key: "lent", label: "Lent" },
    { key: "borrowed", label: "Borrowed" },
    { key: "balances", label: "All balances" },
  ];

  const loans = tab === "lent" ? lentLoans : borrowedLoans;
  const loading = tab === "lent" ? lentLoading : (tab === "borrowed" ? borrowedLoading : balancesLoading);

  return (
    <div className="p-8 space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
          <SheetTrigger asChild>
            <Button size="sm">
              <Plus className="h-4 w-4 mr-1.5" />
              New loan
            </Button>
          </SheetTrigger>
          <SheetContent title="New loan" description="Record a borrow or lend">
            <CreateLoanForm onSuccess={() => setSheetOpen(false)} />
          </SheetContent>
        </Sheet>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b">
        {tabs.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === key
                ? "border-[hsl(var(--primary))] text-[hsl(var(--foreground))]"
                : "border-transparent text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Loan list */}
      {(tab === "lent" || tab === "borrowed") && (
        <>
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-16" />)}
            </div>
          ) : !loans?.length ? (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">
              No {tab} loans yet.
            </p>
          ) : (
            <div className="space-y-2">
              {loans.map((loan) => {
                const statusInfo = LOAN_STATUS_BADGE_SHORT[loan.status] ?? { label: loan.status, variant: "outline" as const };
                return (
                  <Link key={loan.id} href={ROUTES.loanDetail(loan.id) as never}>
                    <div className="p-4 border rounded-xl bg-[hsl(var(--card))] hover:bg-[hsl(var(--muted))] transition-colors cursor-pointer">
                      <div className="flex items-center gap-3">
                        <Avatar name={loan.counterparty_name} size="md" />
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 flex-wrap">
                            <p className="font-medium text-sm">{loan.counterparty_name}</p>
                            <Badge variant={statusInfo.variant} className="text-xs">{statusInfo.label}</Badge>
                          </div>
                          <div className="flex items-center gap-1.5 mt-0.5">
                            {loan.direction === "lent" ? (
                              <ArrowUpRight className="h-3 w-3 text-[hsl(var(--primary))]" />
                            ) : (
                              <ArrowDownLeft className="h-3 w-3 text-[hsl(var(--destructive))]" />
                            )}
                            <span className="text-xs text-[hsl(var(--muted-foreground))]">
                              {loan.direction === "lent" ? "You lent" : "You borrowed"} · <DateDisplay iso={loan.loan_date} />
                            </span>
                          </div>
                          {loan.note && <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5 truncate">{loan.note}</p>}
                        </div>
                        <CurrencyAmount
                          amount={loan.amount}
                          currency={loan.currency}
                          signed={true}
                          className={cn("text-sm font-semibold flex-shrink-0", loan.direction === "lent" ? "text-[hsl(var(--primary))]" : "text-[hsl(var(--destructive))]")}
                        />
                      </div>
                    </div>
                  </Link>
                );
              })}
            </div>
          )}
        </>
      )}

      {/* All balances tab */}
      {tab === "balances" && (
        <>
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-14" />)}
            </div>
          ) : !balances?.length ? (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">
              You&apos;re all settled up across all teams.
            </p>
          ) : (
            <div className="space-y-2">
              {balances.map((b) => (
                <DebtBar
                  key={b.counterparty_id}
                  counterpartyName={b.counterparty_name}
                  netAmount={b.net_amount}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
