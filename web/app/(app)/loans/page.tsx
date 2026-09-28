"use client";

import { useId, useState } from "react";
import Link from "next/link";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowDownLeft, ArrowUpRight, Plus } from "lucide-react";
import { useLoans, useCreateLoan } from "@/hooks/useLoans";
import { useMyBalances } from "@/hooks/useSettlements";
import { DebtBar } from "@/components/settlement/DebtBar";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Avatar } from "@/components/shared/Avatar";
import { PageHeader } from "@/components/shared/PageHeader";
import { QueryBoundary } from "@/components/query-boundary";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useToast } from "@/components/ui/toast";
import { ApiRequestError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { CURRENCY_CODES, LOAN_STATUS_BADGE_SHORT } from "@/constants/config";
import { ROUTES } from "@/constants/routes";
import type { Loan } from "@/types/loan.types";
import type { UseQueryResult } from "@tanstack/react-query";

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
  const { toast } = useToast();
  const [amount, setAmount] = useState(0);
  const [currency, setCurrency] = useState("LKR");
  const [serverError, setServerError] = useState("");
  const fieldId = useId();
  const directionErrorId = `${fieldId}-direction-error`;
  const counterpartyErrorId = `${fieldId}-counterparty-error`;
  const dateErrorId = `${fieldId}-date-error`;
  const amountErrorId = `${fieldId}-amount-error`;
  const {
    register,
    handleSubmit,
    setValue,
    watch,
    control,
    formState: { errors },
  } = useForm<LoanFormValues>({
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
      toast({
        title: "Loan recorded",
        description: `${data.direction === "lent" ? "Lent" : "Borrowed"} ${data.currency} ${data.amount / 100}.`,
        variant: "success",
      });
      onSuccess();
    } catch (err) {
      setServerError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to create loan",
      );
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
      {serverError ? (
        <p
          role="alert"
          className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
        >
          {serverError}
        </p>
      ) : null}
      <fieldset
        className="space-y-1.5"
        aria-describedby={errors.direction ? directionErrorId : undefined}
      >
        <legend className="text-sm font-medium leading-none">Direction</legend>
        <div className="grid grid-cols-2 gap-2">
          {(["lent", "borrowed"] as const).map((direction) => {
            const directionId = `${fieldId}-direction-${direction}`;
            return (
              <label
                key={direction}
                htmlFor={directionId}
                className="cursor-pointer"
              >
                <input
                  id={directionId}
                  type="radio"
                  value={direction}
                  {...register("direction")}
                  className="sr-only"
                />
                <span
                  className={cn(
                    "flex min-h-11 items-center justify-center gap-2 rounded-lg border p-2.5 text-sm font-medium transition-colors",
                    selectedDirection === direction
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-input bg-background text-foreground hover:bg-muted",
                  )}
                >
                  {direction === "lent" ? (
                    <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
                  ) : (
                    <ArrowDownLeft className="h-4 w-4" aria-hidden="true" />
                  )}
                  {direction === "lent" ? "I lent" : "I borrowed"}
                </span>
              </label>
            );
          })}
        </div>
        {errors.direction ? (
          <p id={directionErrorId} className="text-xs text-destructive">
            {errors.direction.message}
          </p>
        ) : null}
      </fieldset>
      <div className="space-y-1.5">
        <Label htmlFor={`${fieldId}-counterparty`}>Person</Label>
        <Input
          id={`${fieldId}-counterparty`}
          placeholder="Name of the other person"
          aria-invalid={errors.counterparty_name ? true : undefined}
          aria-describedby={
            errors.counterparty_name ? counterpartyErrorId : undefined
          }
          {...register("counterparty_name")}
        />
        {errors.counterparty_name ? (
          <p id={counterpartyErrorId} className="text-xs text-destructive">
            {errors.counterparty_name.message}
          </p>
        ) : null}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor={`${fieldId}-currency`}>Currency</Label>
          <Controller
            name="currency"
            control={control}
            render={({ field }) => (
              <Select
                value={field.value}
                onValueChange={(value) => {
                  field.onChange(value);
                  setCurrency(value);
                }}
              >
                <SelectTrigger id={`${fieldId}-currency`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CURRENCY_CODES.map((code) => (
                    <SelectItem key={code} value={code}>
                      {code}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={`${fieldId}-date`}>Date</Label>
          <Input
            id={`${fieldId}-date`}
            type="date"
            aria-invalid={errors.loan_date ? true : undefined}
            aria-describedby={errors.loan_date ? dateErrorId : undefined}
            {...register("loan_date")}
          />
          {errors.loan_date ? (
            <p id={dateErrorId} className="text-xs text-destructive">
              {errors.loan_date.message}
            </p>
          ) : null}
        </div>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${fieldId}-amount`}>Amount</Label>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">{currency}</span>
          <Input
            id={`${fieldId}-amount`}
            type="number"
            inputMode="numeric"
            min="1"
            step="1"
            value={amount || ""}
            onChange={(event) => {
              const next = Number(event.target.value);
              setAmount(Number.isFinite(next) ? next : 0);
              setValue("amount", next, { shouldValidate: true });
            }}
            aria-describedby={errors.amount ? amountErrorId : undefined}
          />
        </div>
        {errors.amount ? (
          <p id={amountErrorId} className="text-xs text-destructive">
            {errors.amount.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${fieldId}-note`}>
          Note <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id={`${fieldId}-note`}
          placeholder="What was this for?"
          {...register("note")}
        />
      </div>
      <Button type="submit" disabled={isPending} className="w-full">
        {isPending ? "Creating…" : "Create loan"}
      </Button>
    </form>
  );
}

type TabKey = "lent" | "borrowed" | "balances";
function isLoanTab(value: string): value is TabKey {
  return value === "lent" || value === "borrowed" || value === "balances";
}

function LoanList({ loans }: { loans: Loan[] }) {
  return (
    <div className="space-y-2">
      {loans.map((loan) => {
        const statusInfo = LOAN_STATUS_BADGE_SHORT[loan.status] ?? {
          label: loan.status,
          variant: "outline" as const,
        };
        return (
          <Link
            key={loan.id}
            href={ROUTES.loanDetail(loan.id) as never}
            className="block rounded-xl border bg-card p-4 transition-colors hover:border-primary/50 hover:bg-muted/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <div className="flex items-center gap-3">
              <Avatar name={loan.counterparty_name} size="md" />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="truncate text-sm font-semibold">
                    {loan.counterparty_name}
                  </p>
                  <Badge variant={statusInfo.variant} className="text-xs">
                    {statusInfo.label}
                  </Badge>
                </div>
                <p className="mt-1 text-xs text-muted-foreground">
                  {loan.direction === "lent" ? "You lent" : "You borrowed"} ·{" "}
                  <DateDisplay iso={loan.loan_date} />
                </p>
                {loan.note ? (
                  <p className="mt-1 truncate text-xs text-muted-foreground">
                    {loan.note}
                  </p>
                ) : null}
              </div>
              <CurrencyAmount
                amount={loan.amount}
                currency={loan.currency}
                signed
                className={cn(
                  "shrink-0 text-sm font-semibold",
                  loan.direction === "lent" ? "text-primary" : "text-negative",
                )}
              />
            </div>
          </Link>
        );
      })}
    </div>
  );
}

function LoanSummary({
  lentQuery,
  borrowedQuery,
}: {
  lentQuery: UseQueryResult<Loan[], Error>;
  borrowedQuery: UseQueryResult<Loan[], Error>;
}) {
  return (
    <QueryBoundary {...lentQuery} isEmpty={() => false} className="min-h-48">
      {(lentData) => (
        <QueryBoundary
          {...borrowedQuery}
          isEmpty={() => false}
          className="min-h-48"
        >
          {(borrowedData) => {
            const totalLent = lentData.reduce(
              (sum, loan) => sum + loan.amount,
              0,
            );
            const totalBorrowed = borrowedData.reduce(
              (sum, loan) => sum + loan.amount,
              0,
            );
            return (
              <div className="grid gap-4 md:grid-cols-[1.2fr_0.8fr]">
                <div className="rounded-xl bg-primary p-6 text-primary-foreground shadow-sm md:p-8">
                  <p className="text-sm text-primary-foreground/75">
                    Net loan position
                  </p>
                  <CurrencyAmount
                    amount={totalLent - totalBorrowed}
                    currency={
                      lentData[0]?.currency ??
                      borrowedData[0]?.currency ??
                      "LKR"
                    }
                    signed
                    className="mt-3 block font-display text-4xl font-semibold text-primary-foreground"
                  />
                  <p className="mt-3 max-w-sm text-sm leading-6 text-primary-foreground/80">
                    Positive means the group owes you more than you owe the
                    group.
                  </p>
                </div>
                <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-1">
                  <div className="rounded-xl border bg-card p-5">
                    <p className="text-sm text-muted-foreground">Total lent</p>
                    <CurrencyAmount
                      amount={totalLent}
                      currency={lentData[0]?.currency ?? "LKR"}
                      className="mt-1 block text-2xl font-semibold"
                    />
                  </div>
                  <div className="rounded-xl border bg-card p-5">
                    <p className="text-sm text-muted-foreground">
                      Total borrowed
                    </p>
                    <CurrencyAmount
                      amount={totalBorrowed}
                      currency={borrowedData[0]?.currency ?? "LKR"}
                      className="mt-1 block text-2xl font-semibold"
                    />
                  </div>
                </div>
              </div>
            );
          }}
        </QueryBoundary>
      )}
    </QueryBoundary>
  );
}

export default function LoansPage() {
  const [tab, setTab] = useState<TabKey>("lent");
  const [sheetOpen, setSheetOpen] = useState(false);
  const lentQuery = useLoans("lent");
  const borrowedQuery = useLoans("borrowed");
  const balancesQuery = useMyBalances();

  return (
    <main className="mx-auto max-w-5xl space-y-8 p-4 md:p-8">
      <PageHeader
        eyebrow="Money between people"
        title="Loans"
        description="Keep direct lending and borrowing visible, even when it never touches a team."
        action={
          <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
            <SheetTrigger asChild>
              <Button>
                <Plus className="h-4 w-4" aria-hidden="true" /> New loan
              </Button>
            </SheetTrigger>
            <SheetContent
              title="New loan"
              description="Record a direct borrow or lend."
            >
              <CreateLoanForm onSuccess={() => setSheetOpen(false)} />
            </SheetContent>
          </Sheet>
        }
      />
      <LoanSummary lentQuery={lentQuery} borrowedQuery={borrowedQuery} />
      <Tabs
        value={tab}
        onValueChange={(value) => {
          if (isLoanTab(value)) setTab(value);
        }}
      >
        <TabsList aria-label="Loan views" className="w-full sm:w-auto">
          <TabsTrigger value="lent">
            Lent{lentQuery.data ? ` (${lentQuery.data.length})` : ""}
          </TabsTrigger>
          <TabsTrigger value="borrowed">
            Borrowed
            {borrowedQuery.data ? ` (${borrowedQuery.data.length})` : ""}
          </TabsTrigger>
          <TabsTrigger value="balances">All balances</TabsTrigger>
        </TabsList>
        <TabsContent value="lent" className="mt-5">
          <QueryBoundary
            {...lentQuery}
            isEmpty={(loans) => loans.length === 0}
            emptyMessage="No lent loans yet. Record one when someone pays you back later."
          >
            <LoanList loans={lentQuery.data ?? []} />
          </QueryBoundary>
        </TabsContent>
        <TabsContent value="borrowed" className="mt-5">
          <QueryBoundary
            {...borrowedQuery}
            isEmpty={(loans) => loans.length === 0}
            emptyMessage="No borrowed loans yet. Record one when you borrow directly."
          >
            <LoanList loans={borrowedQuery.data ?? []} />
          </QueryBoundary>
        </TabsContent>
        <TabsContent value="balances" className="mt-5">
          <QueryBoundary
            {...balancesQuery}
            isEmpty={(balances) => balances.length === 0}
            emptyMessage="You are all settled up across your teams."
          >
            <div className="space-y-2">
              {(balancesQuery.data ?? []).map((balance) => (
                <DebtBar
                  key={balance.counterparty_id}
                  counterpartyName={balance.counterparty_name}
                  netAmount={balance.net_amount}
                />
              ))}
            </div>
          </QueryBoundary>
        </TabsContent>
      </Tabs>
    </main>
  );
}
