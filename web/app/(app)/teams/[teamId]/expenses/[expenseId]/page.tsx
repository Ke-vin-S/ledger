"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useExpense, useCorrectExpense, useVoidExpense } from "@/hooks/useExpenses";
import { useExpenseSettlements, useConfirmSettlement, useDisputeSettlement } from "@/hooks/useSettlements";
import { useExpenseHistory } from "@/hooks/useGraphQL";
import { useTeamMembers } from "@/hooks/useTeam";
import { useMe } from "@/hooks/useAuth";
import { SettlementSheet } from "@/components/settlement/SettlementSheet";
import { AmountInput } from "@/components/expense/AmountInput";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Avatar } from "@/components/shared/Avatar";
import { Skeleton } from "@/components/shared/Skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ChevronDown, ChevronUp, ArrowLeft, Check, X, AlertTriangle } from "lucide-react";
import { ApiRequestError } from "@/lib/api";
import { cn, formatDate } from "@/lib/utils";

const correctionSchema = z.object({
  title: z.string().min(1),
  note: z.string().optional(),
  correction_reason: z.string().min(1, "Reason is required"),
});
type CorrectionValues = z.infer<typeof correctionSchema>;

const STATUS_COLORS: Record<string, string> = {
  pending_confirmation: "text-[hsl(var(--warning,40_96%_40%))] bg-amber-50 dark:bg-amber-950/30 border-amber-200",
  confirmed: "text-[hsl(var(--primary))] bg-blue-50 dark:bg-blue-950/30 border-blue-200",
  disputed: "text-[hsl(var(--destructive))] bg-red-50 dark:bg-red-950/30 border-red-200",
};

export default function ExpenseDetailPage() {
  const { teamId, expenseId } = useParams<{ teamId: string; expenseId: string }>();
  const router = useRouter();
  const { data: me } = useMe();
  const { data: expense, isLoading } = useExpense(expenseId);
  const { data: members } = useTeamMembers(teamId);
  const { data: settlements } = useExpenseSettlements(expenseId);
  const { data: history } = useExpenseHistory(expenseId);
  const { mutateAsync: correctExpense } = useCorrectExpense(teamId);
  const { mutate: voidExpense } = useVoidExpense(teamId);
  const { mutateAsync: confirmSettlement } = useConfirmSettlement();
  const { mutateAsync: disputeSettlement } = useDisputeSettlement();

  const [showHistory, setShowHistory] = useState(false);
  const [showCorrectForm, setShowCorrectForm] = useState(false);
  const [showVoidConfirm, setShowVoidConfirm] = useState(false);
  const [voidReason, setVoidReason] = useState("");
  const [correctionAmount, setCorrectionAmount] = useState(0);
  const [correctionError, setCorrectionError] = useState("");
  const [actionError, setActionError] = useState("");

  const { register, handleSubmit, formState: { errors, isSubmitting }, reset } = useForm<CorrectionValues>({
    resolver: zodResolver(correctionSchema),
    defaultValues: { title: expense?.title ?? "" },
  });

  const memberMap = new Map(members?.map((m) => [m.user_id, m]) ?? []);

  async function onCorrect(data: CorrectionValues) {
    setCorrectionError("");
    try {
      await correctExpense({
        expenseId,
        data: {
          title: data.title,
          amount: correctionAmount || expense?.amount,
          note: data.note,
          correction_reason: data.correction_reason,
        },
      });
      setShowCorrectForm(false);
      reset();
    } catch (err) {
      setCorrectionError(err instanceof ApiRequestError ? err.error.message : "Failed to correct expense");
    }
  }

  async function handleVoid() {
    if (!voidReason.trim()) return;
    voidExpense({ expenseId, reason: voidReason });
    setShowVoidConfirm(false);
    router.push(`/teams/${teamId}`);
  }

  if (isLoading) {
    return (
      <div className="p-8 space-y-4 max-w-2xl">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-48" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  if (!expense) return null;

  const payer = memberMap.get(expense.paid_by);

  return (
    <div className="p-8 space-y-6 max-w-2xl">
      {/* Back */}
      <button
        onClick={() => router.push(`/teams/${teamId}`)}
        className="flex items-center gap-1.5 text-sm text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to team
      </button>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 flex-wrap">
            <h1 className="text-2xl font-bold">{expense.title}</h1>
            {expense.is_void && <Badge variant="secondary">Void</Badge>}
            {expense.version > 1 && <Badge variant="outline">v{expense.version}</Badge>}
          </div>
          <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">
            <DateDisplay iso={expense.expense_date} />
            {expense.note && ` · ${expense.note}`}
          </p>
        </div>
        <CurrencyAmount amount={expense.amount} currency={expense.currency} className="text-xl font-bold flex-shrink-0" />
      </div>

      {/* Paid by */}
      <Card>
        <CardContent className="pt-4">
          <div className="flex items-center gap-3">
            <Avatar name={payer?.display_name ?? expense.paid_by} size="md" />
            <div>
              <p className="text-xs text-[hsl(var(--muted-foreground))]">Paid by</p>
              <p className="text-sm font-medium">{payer?.display_name ?? expense.paid_by}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Splits */}
      {expense.splits && expense.splits.length > 0 && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">Split breakdown</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {expense.splits.map((split) => {
              const member = memberMap.get(split.user_id);
              return (
                <div key={split.id} className="flex items-center gap-3">
                  <Avatar name={member?.display_name ?? split.user_id} size="sm" className="h-6 w-6 text-[0.55rem]" />
                  <span className="flex-1 text-sm">{member?.display_name ?? split.user_id}</span>
                  <CurrencyAmount amount={split.share_amount} currency={expense.currency} className="text-sm font-mono" />
                </div>
              );
            })}
          </CardContent>
        </Card>
      )}

      {/* Settlements */}
      <Card>
        <CardHeader className="pb-3 flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Settlements</CardTitle>
          {!expense.is_void && expense.splits && expense.splits.length > 0 && (
            <SettlementSheet
              teamId={teamId}
              expenseId={expenseId}
              payerId={me?.id ?? ""}
              payeeId={expense.paid_by}
              payeeName={payer?.display_name ?? "Payee"}
              defaultAmount={expense.amount}
              currency={expense.currency}
            >
              <Button size="sm" variant="outline">Record settlement</Button>
            </SettlementSheet>
          )}
        </CardHeader>
        <CardContent>
          {!settlements?.length ? (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">No settlements yet.</p>
          ) : (
            <div className="space-y-2">
              {settlements.map((s) => {
                const payer = memberMap.get(s.payer_id);
                const payee = memberMap.get(s.payee_id);
                const isMe = s.payer_id === me?.id || s.payee_id === me?.id;
                const canAct = s.status === "pending_confirmation" && s.payee_id === me?.id;
                return (
                  <div key={s.id} className={cn("p-3 rounded-lg border text-sm", STATUS_COLORS[s.status] ?? "border-[hsl(var(--border))]")}>
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2 flex-1 min-w-0">
                        <Avatar name={payer?.display_name ?? s.payer_id} size="sm" className="h-5 w-5 text-[0.5rem]" />
                        <span className="truncate text-xs">{payer?.display_name ?? s.payer_id} → {payee?.display_name ?? s.payee_id}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <CurrencyAmount amount={s.amount} currency={expense.currency} className="text-xs font-mono" />
                        <Badge variant="outline" className="text-[0.65rem] py-0 capitalize">{s.status.replace("_", " ")}</Badge>
                      </div>
                    </div>
                    {canAct && (
                      <div className="flex gap-2 mt-2">
                        <Button
                          size="sm"
                          className="h-7 text-xs"
                          onClick={async () => { try { await confirmSettlement(s.id); } catch { setActionError("Failed to confirm"); } }}
                        >
                          <Check className="h-3 w-3 mr-1" /> Confirm
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs"
                          onClick={async () => { try { await disputeSettlement({ settlementId: s.id, reason: "" }); } catch { setActionError("Failed to dispute"); } }}
                        >
                          <X className="h-3 w-3 mr-1" /> Dispute
                        </Button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
          {actionError && <p className="text-xs text-[hsl(var(--destructive))] mt-2">{actionError}</p>}
        </CardContent>
      </Card>

      {/* Version history */}
      <Card>
        <button
          onClick={() => setShowHistory(!showHistory)}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium hover:bg-[hsl(var(--muted)/0.5)] transition-colors rounded-t-xl"
        >
          <span>Version history {history?.expenseHistory.length ? `(${history.expenseHistory.length})` : ""}</span>
          {showHistory ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
        </button>
        {showHistory && (
          <CardContent className="pt-0">
            {!history?.expenseHistory.length ? (
              <p className="text-sm text-[hsl(var(--muted-foreground))]">No corrections recorded.</p>
            ) : (
              <div className="space-y-3">
                {history.expenseHistory.map((v) => (
                  <div key={v.id} className="border-l-2 border-[hsl(var(--border))] pl-3 space-y-1">
                    <p className="text-xs font-medium">v{v.version} — {v.correctionReason ?? "Initial"}</p>
                    <p className="text-xs text-[hsl(var(--muted-foreground))]">
                      {formatDate(v.createdAt)}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        )}
      </Card>

      {/* Actions */}
      {!expense.is_void && (
        <div className="flex gap-3 flex-wrap">
          <Button variant="outline" size="sm" onClick={() => { setShowCorrectForm(!showCorrectForm); setCorrectionAmount(expense.amount); reset({ title: expense.title, note: expense.note, correction_reason: "" }); }}>
            Correct expense
          </Button>
          <Button variant="outline" size="sm" className="text-[hsl(var(--destructive))] border-[hsl(var(--destructive)/0.3)] hover:bg-[hsl(var(--destructive)/0.05)]" onClick={() => setShowVoidConfirm(!showVoidConfirm)}>
            Void expense
          </Button>
        </div>
      )}

      {/* Correction form */}
      {showCorrectForm && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">Correct expense</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit(onCorrect)} className="space-y-3">
              {correctionError && <p className="text-xs text-[hsl(var(--destructive))]">{correctionError}</p>}
              <div className="space-y-1">
                <Label className="text-xs">Title</Label>
                <Input {...register("title")} className="h-8 text-sm" />
                {errors.title && <p className="text-xs text-[hsl(var(--destructive))]">{errors.title.message}</p>}
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Amount</Label>
                <AmountInput value={correctionAmount} currency={expense.currency} onChange={setCorrectionAmount} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Note</Label>
                <Input {...register("note")} className="h-8 text-sm" placeholder="Optional note" />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Reason for correction <span className="text-[hsl(var(--destructive))]">*</span></Label>
                <Input {...register("correction_reason")} className="h-8 text-sm" placeholder="Why is this being corrected?" />
                {errors.correction_reason && <p className="text-xs text-[hsl(var(--destructive))]">{errors.correction_reason.message}</p>}
              </div>
              <div className="flex gap-2">
                <Button type="submit" size="sm" disabled={isSubmitting}>
                  {isSubmitting ? "Saving…" : "Save correction"}
                </Button>
                <Button type="button" variant="ghost" size="sm" onClick={() => setShowCorrectForm(false)}>Cancel</Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {/* Void confirm */}
      {showVoidConfirm && (
        <Card className="border-[hsl(var(--destructive)/0.3)]">
          <CardContent className="pt-4 space-y-3">
            <div className="flex items-center gap-2 text-[hsl(var(--destructive))]">
              <AlertTriangle className="h-4 w-4" />
              <p className="text-sm font-medium">Void this expense?</p>
            </div>
            <p className="text-xs text-[hsl(var(--muted-foreground))]">This will mark the expense as void. The record is kept for audit purposes.</p>
            <div className="space-y-1">
              <Label className="text-xs">Reason <span className="text-[hsl(var(--destructive))]">*</span></Label>
              <Input
                placeholder="Why is this being voided?"
                value={voidReason}
                onChange={(e) => setVoidReason(e.target.value)}
                className="h-8 text-sm"
              />
            </div>
            <div className="flex gap-2">
              <Button variant="destructive" size="sm" onClick={handleVoid} disabled={!voidReason.trim()}>
                Confirm void
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setShowVoidConfirm(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
