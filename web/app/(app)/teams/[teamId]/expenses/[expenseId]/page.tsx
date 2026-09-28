"use client";
import { useId, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  useExpense,
  useCorrectExpense,
  useVoidExpense,
} from "@/hooks/useExpenses";
import {
  useExpenseSettlements,
  useConfirmSettlement,
  useDisputeSettlement,
} from "@/hooks/useSettlements";
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
import {
  ChevronDown,
  ChevronUp,
  ArrowLeft,
  Check,
  X,
  AlertTriangle,
} from "lucide-react";
import { ApiRequestError } from "@/lib/api";
import { cn, formatDate } from "@/lib/utils";
import { SETTLEMENT_STATUS_COLORS } from "@/constants/config";
import { ROUTES } from "@/constants/routes";

const correctionSchema = z.object({
  title: z.string().min(1),
  note: z.string().optional(),
  correction_reason: z.string().min(1, "Reason is required"),
});
type CorrectionValues = z.infer<typeof correctionSchema>;

export default function ExpenseDetailPage() {
  const { teamId, expenseId } = useParams<{
    teamId: string;
    expenseId: string;
  }>();
  const router = useRouter();
  const { data: me } = useMe();
  const formId = useId();
  const { data: expense, isLoading } = useExpense(expenseId);
  const { data: members } = useTeamMembers(teamId);
  const { data: settlements } = useExpenseSettlements(expenseId);
  const { data: history } = useExpenseHistory(expenseId);
  const { mutateAsync: correctExpense } = useCorrectExpense(teamId);
  const { mutateAsync: voidExpense, isPending: isVoiding } =
    useVoidExpense(teamId);
  const { mutateAsync: confirmSettlement } = useConfirmSettlement(
    teamId,
    expenseId,
  );
  const { mutateAsync: disputeSettlement } = useDisputeSettlement(
    teamId,
    expenseId,
  );

  const [showHistory, setShowHistory] = useState(false);
  const [showCorrectForm, setShowCorrectForm] = useState(false);
  const [showVoidConfirm, setShowVoidConfirm] = useState(false);
  const [voidReason, setVoidReason] = useState("");
  const [correctionAmount, setCorrectionAmount] = useState(0);
  const [correctionError, setCorrectionError] = useState("");
  const [actionError, setActionError] = useState("");

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<CorrectionValues>({
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
      setCorrectionError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to correct expense",
      );
    }
  }

  async function handleVoid() {
    if (!voidReason.trim()) return;
    setActionError("");
    try {
      await voidExpense({ expenseId, reason: voidReason.trim() });
      setShowVoidConfirm(false);
      router.push(ROUTES.team(teamId) as never);
    } catch (err) {
      setActionError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to void expense",
      );
    }
  }

  if (isLoading) {
    return (
      <div className="p-4 md:p-8 space-y-4 max-w-2xl">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-48" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  if (!expense) {
    return (
      <div className="p-4 md:p-8 max-w-2xl">
        <Card>
          <CardContent className="flex flex-col items-start gap-4 py-8">
            <div>
              <h1 className="text-xl font-semibold">Expense not found</h1>
              <p className="mt-1 text-sm text-muted-foreground">
                This expense may have been removed or you may not have access to
                it.
              </p>
            </div>
            <Button onClick={() => router.push(ROUTES.team(teamId) as never)}>
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to team
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const payer = memberMap.get(expense.paid_by);

  return (
    <div className="p-4 md:p-8 space-y-6 max-w-2xl">
      {/* Back */}
      <button
        type="button"
        onClick={() => router.push(ROUTES.team(teamId) as never)}
        className="min-h-9 inline-flex items-center gap-1.5 px-2 -ml-2 rounded-md text-sm text-muted-foreground hover:text-foreground transition-colors"
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
            {expense.version > 1 && (
              <Badge variant="outline">v{expense.version}</Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            <DateDisplay iso={expense.expense_date} />
            {expense.note && ` · ${expense.note}`}
          </p>
        </div>
        <CurrencyAmount
          amount={expense.amount}
          currency={expense.currency}
          className="text-xl font-bold flex-shrink-0"
        />
      </div>

      {/* Paid by */}
      <Card>
        <CardContent className="pt-4">
          <div className="flex items-center gap-3">
            <Avatar name={payer?.display_name ?? expense.paid_by} size="md" />
            <div>
              <p className="text-xs text-muted-foreground">Paid by</p>
              <p className="text-sm font-medium">
                {payer?.display_name ?? expense.paid_by}
              </p>
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
                  <Avatar
                    name={member?.display_name ?? split.user_id}
                    size="sm"
                    className="h-6 w-6 text-xs"
                  />
                  <span className="flex-1 text-sm">
                    {member?.display_name ?? split.user_id}
                  </span>
                  <CurrencyAmount
                    amount={split.share_amount}
                    currency={expense.currency}
                    className="text-sm font-mono"
                  />
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
              <Button size="sm" variant="outline" className="min-h-9">
                Record settlement
              </Button>
            </SettlementSheet>
          )}
        </CardHeader>
        <CardContent>
          {!settlements?.length ? (
            <p className="text-sm text-muted-foreground">No settlements yet.</p>
          ) : (
            <div className="space-y-2">
              {settlements.map((s) => {
                const payer = memberMap.get(s.payer_id);
                const payee = memberMap.get(s.payee_id);
                const canAct =
                  s.status === "pending_confirmation" && s.payee_id === me?.id;
                return (
                  <div
                    key={s.id}
                    className={cn(
                      "p-3 rounded-lg border text-sm",
                      SETTLEMENT_STATUS_COLORS[s.status] ?? "border-border",
                    )}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2 flex-1 min-w-0">
                        <Avatar
                          name={payer?.display_name ?? s.payer_id}
                          size="sm"
                          className="h-5 w-5 text-xs"
                        />
                        <span className="truncate text-xs">
                          {payer?.display_name ?? s.payer_id} →{" "}
                          {payee?.display_name ?? s.payee_id}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <CurrencyAmount
                          amount={s.amount}
                          currency={expense.currency}
                          className="text-xs font-mono"
                        />
                        <Badge
                          variant="outline"
                          className="text-xs py-0 capitalize"
                        >
                          {s.status.replace("_", " ")}
                        </Badge>
                      </div>
                    </div>
                    {canAct && (
                      <div className="flex gap-2 mt-2">
                        <Button
                          size="sm"
                          className="min-h-9 text-xs"
                          onClick={async () => {
                            try {
                              await confirmSettlement(s.id);
                            } catch {
                              setActionError("Failed to confirm");
                            }
                          }}
                        >
                          <Check className="h-3 w-3 mr-1" /> Confirm
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="min-h-9 text-xs"
                          onClick={async () => {
                            try {
                              await disputeSettlement({
                                settlementId: s.id,
                                reason: "",
                              });
                            } catch {
                              setActionError("Failed to dispute");
                            }
                          }}
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
          {actionError && (
            <p className="text-xs text-destructive mt-2">{actionError}</p>
          )}
        </CardContent>
      </Card>

      {/* Version history */}
      <Card>
        <button
          type="button"
          aria-expanded={showHistory}
          aria-controls={`${formId}-history`}
          onClick={() => setShowHistory(!showHistory)}
          className="min-h-9 w-full flex items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors rounded-t-xl"
        >
          <span>
            Version history{" "}
            {history?.expenseHistory.length
              ? `(${history.expenseHistory.length})`
              : ""}
          </span>
          {showHistory ? (
            <ChevronUp className="h-4 w-4" />
          ) : (
            <ChevronDown className="h-4 w-4" />
          )}
        </button>
        {showHistory && (
          <CardContent id={`${formId}-history`} className="pt-0">
            {!history?.expenseHistory.length ? (
              <p className="text-sm text-muted-foreground">
                No corrections recorded.
              </p>
            ) : (
              <div className="space-y-3">
                {history.expenseHistory.map((v) => (
                  <div
                    key={v.id}
                    className="border-l-2 border-border pl-3 space-y-1"
                  >
                    <p className="text-xs font-medium">
                      v{v.version} — {v.correctionReason ?? "Initial"}
                    </p>
                    <p className="text-xs text-muted-foreground">
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
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="min-h-9"
            aria-expanded={showCorrectForm}
            aria-controls={`${formId}-correction-form`}
            onClick={() => {
              setShowCorrectForm(!showCorrectForm);
              setCorrectionAmount(expense.amount);
              reset({
                title: expense.title,
                note: expense.note,
                correction_reason: "",
              });
            }}
          >
            Correct expense
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="min-h-9 text-destructive border-destructive/30 hover:bg-destructive/5"
            aria-expanded={showVoidConfirm}
            aria-controls={`${formId}-void-form`}
            onClick={() => setShowVoidConfirm(!showVoidConfirm)}
          >
            Void expense
          </Button>
        </div>
      )}

      {/* Correction form */}
      {showCorrectForm && (
        <Card id={`${formId}-correction-form`}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">Correct expense</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit(onCorrect)} className="space-y-3">
              {correctionError && (
                <p className="text-xs text-destructive">{correctionError}</p>
              )}
              <div className="space-y-1">
                <Label
                  className="text-xs"
                  htmlFor={`${formId}-correction-title`}
                >
                  Title
                </Label>
                <Input
                  id={`${formId}-correction-title`}
                  {...register("title")}
                  className="h-9 text-sm"
                />
                {errors.title && (
                  <p className="text-xs text-destructive">
                    {errors.title.message}
                  </p>
                )}
              </div>
              <div className="space-y-1">
                <Label
                  className="text-xs"
                  htmlFor={`${formId}-correction-amount`}
                >
                  Amount
                </Label>
                <AmountInput
                  id={`${formId}-correction-amount`}
                  value={correctionAmount}
                  currency={expense.currency}
                  onChange={setCorrectionAmount}
                />
              </div>
              <div className="space-y-1">
                <Label
                  className="text-xs"
                  htmlFor={`${formId}-correction-note`}
                >
                  Note
                </Label>
                <Input
                  id={`${formId}-correction-note`}
                  {...register("note")}
                  className="h-9 text-sm"
                  placeholder="Optional note"
                />
              </div>
              <div className="space-y-1">
                <Label
                  className="text-xs"
                  htmlFor={`${formId}-correction-reason`}
                >
                  Reason for correction{" "}
                  <span className="text-destructive">*</span>
                </Label>
                <Input
                  id={`${formId}-correction-reason`}
                  {...register("correction_reason")}
                  className="h-9 text-sm"
                  placeholder="Why is this being corrected?"
                />
                {errors.correction_reason && (
                  <p className="text-xs text-destructive">
                    {errors.correction_reason.message}
                  </p>
                )}
              </div>
              <div className="flex gap-2">
                <Button
                  type="submit"
                  size="sm"
                  className="min-h-9"
                  disabled={isSubmitting}
                >
                  {isSubmitting ? "Saving…" : "Save correction"}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="min-h-9"
                  onClick={() => setShowCorrectForm(false)}
                >
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {/* Void confirm */}
      {showVoidConfirm && (
        <Card id={`${formId}-void-form`} className="border-destructive/30">
          <CardContent className="pt-4 space-y-3">
            <div className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-4 w-4" />
              <p className="text-sm font-medium">Void this expense?</p>
            </div>
            <p className="text-xs text-muted-foreground">
              This will mark the expense as void. The record is kept for audit
              purposes.
            </p>
            <div className="space-y-1">
              <Label className="text-xs" htmlFor={`${formId}-void-reason`}>
                Reason <span className="text-destructive">*</span>
              </Label>
              <Input
                id={`${formId}-void-reason`}
                placeholder="Why is this being voided?"
                value={voidReason}
                onChange={(e) => setVoidReason(e.target.value)}
                className="h-9 text-sm"
              />
            </div>
            {actionError && (
              <p role="alert" className="text-xs text-destructive">
                {actionError}
              </p>
            )}
            <div className="flex gap-2">
              <Button
                type="button"
                variant="destructive"
                size="sm"
                className="min-h-9"
                onClick={handleVoid}
                disabled={!voidReason.trim() || isVoiding}
              >
                {isVoiding ? "Voiding…" : "Confirm void"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="min-h-9"
                onClick={() => setShowVoidConfirm(false)}
              >
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
