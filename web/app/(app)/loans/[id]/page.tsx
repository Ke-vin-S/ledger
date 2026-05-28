"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useLoan, useAcknowledgeLoan, useDisputeLoan, useLoanClaimText } from "@/hooks/useLoans";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Avatar } from "@/components/shared/Avatar";
import { Skeleton } from "@/components/shared/Skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ArrowLeft, ArrowUpRight, ArrowDownLeft, Copy, Check } from "lucide-react";
import { ApiRequestError } from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { LOAN_STATUS_BADGE } from "@/constants/config";
import { ROUTES } from "@/constants/routes";

export default function LoanDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data: loan, isLoading } = useLoan(id);
  const { mutateAsync: acknowledge, isPending: ackPending } = useAcknowledgeLoan();
  const { mutateAsync: dispute, isPending: disputePending } = useDisputeLoan();
  const { mutateAsync: getClaimText } = useLoanClaimText();

  const [disputeReason, setDisputeReason] = useState("");
  const [showDisputeInput, setShowDisputeInput] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

  async function handleAcknowledge() {
    try {
      await acknowledge(id);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.error.message : "Failed to acknowledge");
    }
  }

  async function handleDispute() {
    try {
      await dispute({ loanId: id, reason: disputeReason || undefined });
      setShowDisputeInput(false);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.error.message : "Failed to dispute");
    }
  }

  async function handleCopyReminder() {
    try {
      const res = await getClaimText(id);
      await navigator.clipboard.writeText(res.text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* ignore */ }
  }

  if (isLoading) {
    return (
      <div className="p-8 space-y-4 max-w-lg">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-40" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  if (!loan) return null;

  const statusInfo = LOAN_STATUS_BADGE[loan.status] ?? { label: loan.status, variant: "outline" as const };
  const repaid = loan.repayments?.reduce((s, r) => s + r.amount, 0) ?? 0;
  const outstanding = loan.amount - repaid;

  return (
    <div className="p-8 space-y-6 max-w-lg">
      {/* Back */}
      <button
        onClick={() => router.push(ROUTES.loans)}
        className="flex items-center gap-1.5 text-sm text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to loans
      </button>

      {/* Header */}
      <Card>
        <CardContent className="pt-5 space-y-4">
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-center gap-3">
              <Avatar name={loan.counterparty_name} size="md" />
              <div>
                <p className="font-semibold">{loan.counterparty_name}</p>
                <div className="flex items-center gap-1.5 mt-0.5">
                  {loan.direction === "lent" ? (
                    <ArrowUpRight className="h-3.5 w-3.5 text-[hsl(var(--primary))]" />
                  ) : (
                    <ArrowDownLeft className="h-3.5 w-3.5 text-[hsl(var(--destructive))]" />
                  )}
                  <span className="text-xs text-[hsl(var(--muted-foreground))]">
                    {loan.direction === "lent" ? "You lent" : "You borrowed"} · <DateDisplay iso={loan.loan_date} />
                  </span>
                </div>
              </div>
            </div>
            <Badge variant={statusInfo.variant}>{statusInfo.label}</Badge>
          </div>

          <div className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <p className="text-xs text-[hsl(var(--muted-foreground))]">Original amount</p>
              <CurrencyAmount amount={loan.amount} currency={loan.currency} className="font-semibold text-base" />
            </div>
            {repaid > 0 && (
              <div>
                <p className="text-xs text-[hsl(var(--muted-foreground))]">Repaid</p>
                <CurrencyAmount amount={repaid} currency={loan.currency} className="font-semibold text-base" />
              </div>
            )}
            {outstanding > 0 && outstanding !== loan.amount && (
              <div>
                <p className="text-xs text-[hsl(var(--muted-foreground))]">Outstanding</p>
                <CurrencyAmount amount={outstanding} currency={loan.currency} className="font-semibold text-base text-[hsl(var(--destructive))]" />
              </div>
            )}
          </div>

          {loan.note && (
            <p className="text-sm text-[hsl(var(--muted-foreground))] border-t pt-3">{loan.note}</p>
          )}
        </CardContent>
      </Card>

      {/* Actions */}
      {loan.status === "outstanding" && (
        <div className="flex gap-2 flex-wrap">
          <Button size="sm" onClick={handleAcknowledge} disabled={ackPending}>
            {ackPending ? "Acknowledging…" : "Acknowledge"}
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="text-[hsl(var(--destructive))]"
            onClick={() => setShowDisputeInput(!showDisputeInput)}
          >
            Dispute
          </Button>
          <Button size="sm" variant="outline" onClick={handleCopyReminder}>
            {copied ? <><Check className="h-3.5 w-3.5 mr-1.5" /> Copied!</> : <><Copy className="h-3.5 w-3.5 mr-1.5" /> Copy reminder</>}
          </Button>
        </div>
      )}

      {showDisputeInput && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">Reason <span className="text-[hsl(var(--muted-foreground))]">(optional)</span></Label>
              <Input
                placeholder="Why are you disputing this?"
                value={disputeReason}
                onChange={(e) => setDisputeReason(e.target.value)}
                className="h-8 text-sm"
              />
            </div>
            <div className="flex gap-2">
              <Button size="sm" variant="destructive" onClick={handleDispute} disabled={disputePending}>
                {disputePending ? "Disputing…" : "Confirm dispute"}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setShowDisputeInput(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {error && <p className="text-sm text-[hsl(var(--destructive))]">{error}</p>}

      {/* Repayment timeline */}
      {loan.repayments && loan.repayments.length > 0 && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">Repayment timeline</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {loan.repayments.map((r) => (
              <div key={r.id} className="flex items-center gap-3 border-l-2 border-[hsl(var(--primary)/0.3)] pl-3">
                <div className="flex-1 min-w-0">
                  <DateDisplay iso={r.repaid_at} />
                  {r.note && <p className="text-xs text-[hsl(var(--muted-foreground))] truncate">{r.note}</p>}
                </div>
                <CurrencyAmount amount={r.amount} currency={loan.currency} className="text-sm font-mono" />
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
