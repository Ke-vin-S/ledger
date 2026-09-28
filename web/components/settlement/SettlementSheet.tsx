"use client";
import { useId, useState } from "react";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AmountInput } from "@/components/expense/AmountInput";
import { Avatar } from "@/components/shared/Avatar";
import { useRecordSettlement } from "@/hooks/useSettlements";
import { ApiRequestError } from "@/lib/api";
import { SETTLEMENT_METHODS } from "@/constants/config";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { SettlementMethod } from "@/types/settlement.types";

type Method = SettlementMethod;

type Props = {
  teamId: string;
  expenseId: string;
  payerId: string;
  payeeId: string;
  payeeName: string;
  defaultAmount: number;
  currency: string;
  children: React.ReactNode;
};

export function SettlementSheet({
  teamId,
  expenseId,
  payerId,
  payeeId,
  payeeName,
  defaultAmount,
  currency,
  children,
}: Props) {
  const [open, setOpen] = useState(false);
  const formId = useId();
  const [amount, setAmount] = useState(defaultAmount);
  const [method, setMethod] = useState<Method>("cash");
  const [methodNote, setMethodNote] = useState("");
  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  const [error, setError] = useState("");

  const { mutateAsync, isPending } = useRecordSettlement(teamId, expenseId);

  async function handleSubmit() {
    if (!amount) {
      setError("Amount is required");
      return;
    }
    setError("");
    try {
      await mutateAsync({
        payer_id: payerId,
        payee_id: payeeId,
        amount,
        method,
        method_note: methodNote || undefined,
        settled_on: date,
      });
      setOpen(false);
    } catch (err) {
      setError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to record settlement",
      );
    }
  }

  const needsNote = method === "upi" || method === "bank_transfer";

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>{children}</SheetTrigger>
      <SheetContent
        title="Record settlement"
        description={`Settle with ${payeeName}`}
      >
        <div className="space-y-5">
          {/* Counterparty */}
          <div className="flex items-center gap-3 p-3 rounded-xl bg-muted/50">
            <Avatar name={payeeName} size="md" />
            <div>
              <p className="text-sm font-medium">{payeeName}</p>
              <p className="text-xs text-muted-foreground">
                Marking as settled
              </p>
            </div>
          </div>

          {error && (
            <div
              role="alert"
              className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
            >
              {error}
            </div>
          )}

          {/* Amount */}
          <div className="space-y-1.5">
            <Label htmlFor={`${formId}-amount`}>Amount</Label>
            <AmountInput
              id={`${formId}-amount`}
              value={amount}
              currency={currency}
              onChange={setAmount}
            />
          </div>

          {/* Method */}
          <div className="space-y-1.5">
            <Label htmlFor={`${formId}-method`}>Payment method</Label>
            <Select
              value={method}
              onValueChange={(v) => setMethod(v as Method)}
            >
              <SelectTrigger id={`${formId}-method`}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SETTLEMENT_METHODS.map(({ value, label }) => (
                  <SelectItem key={value} value={value}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Method note (UPI/bank) */}
          {needsNote && (
            <div className="space-y-1.5">
              <Label htmlFor={`${formId}-reference`}>Reference number</Label>
              <Input
                id={`${formId}-reference`}
                placeholder={
                  method === "upi" ? "UPI transaction ID" : "Bank reference"
                }
                value={methodNote}
                onChange={(e) => setMethodNote(e.target.value)}
              />
            </div>
          )}

          {/* Date */}
          <div className="space-y-1.5">
            <Label htmlFor={`${formId}-date`}>Date</Label>
            <Input
              id={`${formId}-date`}
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
            />
          </div>

          <div className="flex gap-3 pt-2">
            <Button
              type="button"
              onClick={handleSubmit}
              disabled={isPending}
              className="flex-1"
            >
              {isPending ? "Recording…" : "Record settlement"}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
          </div>

          <p className="text-xs text-muted-foreground text-center">
            The other party will need to confirm this settlement.
          </p>
        </div>
      </SheetContent>
    </Sheet>
  );
}
