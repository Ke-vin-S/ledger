"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Copy } from "lucide-react";
import {
  useAcknowledgeLoan,
  useDisputeLoan,
  useLoan,
  useLoanClaimText,
} from "@/hooks/useLoans";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Avatar } from "@/components/shared/Avatar";
import { PageHeader } from "@/components/shared/PageHeader";
import { QueryBoundary } from "@/components/query-boundary";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { ApiRequestError } from "@/lib/api";
import { LOAN_STATUS_BADGE } from "@/constants/config";
import { ROUTES } from "@/constants/routes";
import type { Loan } from "@/types/loan.types";

function LoanActions({ loan }: { loan: Loan }) {
  const acknowledge = useAcknowledgeLoan();
  const dispute = useDisputeLoan();
  const getClaimText = useLoanClaimText();
  const { toast } = useToast();
  const [disputeOpen, setDisputeOpen] = useState(false);
  const [reason, setReason] = useState("");

  async function handleAcknowledge() {
    try {
      await acknowledge.mutateAsync(loan.id);
      toast({ title: "Loan acknowledged", variant: "success" });
    } catch (err) {
      toast({
        title: "Could not acknowledge loan",
        description:
          err instanceof ApiRequestError ? err.error.message : "Try again.",
        variant: "destructive",
      });
    }
  }

  async function handleDispute() {
    try {
      await dispute.mutateAsync({
        loanId: loan.id,
        reason: reason || undefined,
      });
      toast({
        title: "Loan disputed",
        description: "The other person will be notified.",
        variant: "success",
      });
      setDisputeOpen(false);
      setReason("");
    } catch (err) {
      toast({
        title: "Could not dispute loan",
        description:
          err instanceof ApiRequestError ? err.error.message : "Try again.",
        variant: "destructive",
      });
    }
  }

  async function handleCopyReminder() {
    try {
      const result = await getClaimText.mutateAsync(loan.id);
      await navigator.clipboard.writeText(result.text);
      toast({
        title: "Reminder copied",
        description: "Share it with the other person.",
        variant: "success",
      });
    } catch (err) {
      toast({
        title: "Could not copy reminder",
        description:
          err instanceof ApiRequestError ? err.error.message : "Try again.",
        variant: "destructive",
      });
    }
  }

  if (loan.status !== "outstanding") return null;

  return (
    <>
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          onClick={handleAcknowledge}
          disabled={acknowledge.isPending}
        >
          {acknowledge.isPending ? "Acknowledging…" : "Acknowledge"}
        </Button>
        <Dialog open={disputeOpen} onOpenChange={setDisputeOpen}>
          <DialogTrigger asChild>
            <Button
              type="button"
              variant="outline"
              className="text-destructive"
            >
              Dispute
            </Button>
          </DialogTrigger>
          <DialogContent
            title="Dispute this loan"
            description="Explain what is wrong. The other person will see your reason."
          >
            <form
              onSubmit={(event) => {
                event.preventDefault();
                void handleDispute();
              }}
              className="space-y-4"
            >
              <div className="space-y-1.5">
                <Label htmlFor="loan-dispute-reason">
                  Reason{" "}
                  <span className="text-muted-foreground">(optional)</span>
                </Label>
                <Input
                  id="loan-dispute-reason"
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  placeholder="Why are you disputing this?"
                />
              </div>
              <DialogFooter>
                <Button
                  type="submit"
                  variant="destructive"
                  disabled={dispute.isPending}
                >
                  {dispute.isPending ? "Disputing…" : "Confirm dispute"}
                </Button>
                <DialogClose asChild>
                  <Button type="button" variant="outline">
                    Cancel
                  </Button>
                </DialogClose>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
        <Button
          type="button"
          variant="outline"
          onClick={handleCopyReminder}
          disabled={getClaimText.isPending}
        >
          <Copy className="h-4 w-4" aria-hidden="true" /> Copy reminder
        </Button>
      </div>
    </>
  );
}

function LoanDetail({ loan }: { loan: Loan }) {
  const statusInfo = LOAN_STATUS_BADGE[loan.status] ?? {
    label: loan.status,
    variant: "outline" as const,
  };
  const repaid =
    loan.repayments?.reduce((sum, repayment) => sum + repayment.amount, 0) ?? 0;
  const outstanding = loan.amount - repaid;
  const directionLabel =
    loan.direction === "lent" ? "You lent" : "You borrowed";

  return (
    <div className="space-y-8">
      <Link
        href={ROUTES.loans}
        className="inline-flex min-h-9 items-center gap-2 rounded-md px-2 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" /> Back to loans
      </Link>
      <PageHeader
        eyebrow="Direct loan"
        title={loan.counterparty_name}
        description={`${directionLabel} · ${new Date(loan.loan_date).toLocaleDateString("en-US", { dateStyle: "medium" })}`}
        action={
          <Badge variant={statusInfo.variant} className="w-fit">
            {statusInfo.label}
          </Badge>
        }
      />
      <section className="grid gap-4 md:grid-cols-[1.2fr_0.8fr]">
        <Card className="border-primary/30 bg-primary text-primary-foreground">
          <CardContent className="p-6 md:p-8">
            <div className="flex items-center gap-3">
              <Avatar name={loan.counterparty_name} size="lg" />
              <div>
                <p className="text-sm text-primary-foreground/75">
                  {directionLabel}
                </p>
                <p className="font-medium">{loan.currency} amount</p>
              </div>
            </div>
            <CurrencyAmount
              amount={loan.amount}
              currency={loan.currency}
              className="mt-8 block font-display text-4xl font-semibold tracking-tight text-primary-foreground"
            />
            {loan.note ? (
              <p className="mt-6 border-t border-primary-foreground/20 pt-4 text-sm leading-6 text-primary-foreground/80">
                {loan.note}
              </p>
            ) : null}
          </CardContent>
        </Card>
        <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-1">
          <Card>
            <CardContent className="p-5">
              <p className="text-sm text-muted-foreground">Original amount</p>
              <CurrencyAmount
                amount={loan.amount}
                currency={loan.currency}
                className="mt-1 block text-2xl font-semibold"
              />
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-5">
              <p className="text-sm text-muted-foreground">Outstanding</p>
              <CurrencyAmount
                amount={Math.max(outstanding, 0)}
                currency={loan.currency}
                className="mt-1 block text-2xl font-semibold"
              />
            </CardContent>
          </Card>
        </div>
      </section>
      <LoanActions loan={loan} />
      {loan.repayments && loan.repayments.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Repayment timeline</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableCaption className="sr-only">
                Repayments for this loan
              </TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Note</TableHead>
                  <TableHead className="text-right">Amount</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loan.repayments.map((repayment) => (
                  <TableRow key={repayment.id}>
                    <TableCell>
                      <DateDisplay iso={repayment.repaid_at} />
                    </TableCell>
                    <TableCell>{repayment.note || "—"}</TableCell>
                    <TableCell className="text-right">
                      <CurrencyAmount
                        amount={repayment.amount}
                        currency={loan.currency}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

export default function LoanDetailPage() {
  const { id } = useParams<{ id: string }>();
  const query = useLoan(id);
  return (
    <main className="mx-auto max-w-4xl p-4 md:p-8">
      <QueryBoundary {...query} isEmpty={() => false} className="min-h-64">
        {(loan) => <LoanDetail loan={loan} />}
      </QueryBoundary>
    </main>
  );
}
