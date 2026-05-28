"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Sheet, SheetTrigger, SheetContent, SheetClose } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AmountInput } from "@/components/expense/AmountInput";
import { MemberPicker } from "@/components/expense/MemberPicker";
import { SplitBuilder } from "@/components/expense/SplitBuilder";
import { useTeams, useTeamMembers, isAnonymousMember } from "@/hooks/useTeam";
import { useMe } from "@/hooks/useAuth";
import { useCreateExpense } from "@/hooks/useExpenses";
import { ApiRequestError } from "@/lib/api";
import { CURRENCIES, SPLIT_METHODS, SELECT_CLASS } from "@/constants/config";
import type { SplitMethod, SplitEntry } from "@/types/expense.types";
import type { PickedMember } from "@/types/team.types";

const schema = z.object({
  team_id: z.string().min(1, "Select a team"),
  title: z.string().min(1, "Title is required"),
  amount: z.number().int().positive("Amount must be positive"),
  currency: z.string().min(1),
  split_method: z.enum(["equal", "exact", "percentage", "shares"]),
  expense_date: z.string().min(1, "Date is required"),
  paid_by: z.string().min(1, "Select who paid"),
  note: z.string().optional(),
});
type FormValues = z.infer<typeof schema>;

function AddExpenseForm({ onSuccess }: { onSuccess: () => void }) {
  const { data: teams } = useTeams();
  const { data: me } = useMe();
  const [selectedTeamId, setSelectedTeamId] = useState<string>("");
  const [amount, setAmount] = useState(0);
  const [currency, setCurrency] = useState("LKR");
  const [splitMethod, setSplitMethod] = useState<SplitMethod>("equal");
  const [participants, setParticipants] = useState<string[]>([]);
  const [participantObjects, setParticipantObjects] = useState<PickedMember[]>([]);
  const [splits, setSplits] = useState<SplitEntry[]>([]);
  const [serverError, setServerError] = useState<string | null>(null);

  const { data: teamMembers } = useTeamMembers(selectedTeamId);
  const { mutateAsync, isPending } = useCreateExpense(selectedTeamId);

  const {
    register,
    handleSubmit,
    setValue,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      currency: "LKR",
      split_method: "equal",
      expense_date: new Date().toISOString().split("T")[0],
    },
  });

  function handleTeamChange(teamId: string) {
    setSelectedTeamId(teamId);
    setValue("team_id", teamId);
    setParticipants([]);
    setParticipantObjects([]);
    setSplits([]);
    if (me) setValue("paid_by", me.id);
  }

  async function onSubmit(data: FormValues) {
    if (!data.team_id) return;
    if (participants.length === 0) {
      setServerError("Select at least one participant to split with.");
      return;
    }
    setServerError(null);
    try {
      const payload: Parameters<typeof mutateAsync>[0] = {
        title: data.title,
        amount: data.amount,
        currency: data.currency,
        split_method: data.split_method,
        expense_date: data.expense_date,
        paid_by: data.paid_by,
        note: data.note || undefined,
        splits: splits.length > 0 ? splits : participants.map((id) => ({ user_id: id })),
      };
      await mutateAsync(payload);
      reset();
      setAmount(0);
      setParticipants([]);
      setParticipantObjects([]);
      setSplits([]);
      setSelectedTeamId("");
      onSuccess();
    } catch (err) {
      if (err instanceof ApiRequestError) setServerError(err.error.message);
      else setServerError("Failed to add expense.");
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
      {serverError && (
        <div className="p-3 rounded-lg bg-[hsl(var(--destructive)/0.1)] text-[hsl(var(--destructive))] text-sm">
          {serverError}
        </div>
      )}

      {/* Team */}
      <div className="space-y-1.5">
        <Label>Team</Label>
        <select
          className={SELECT_CLASS}
          {...register("team_id")}
          onChange={(e) => handleTeamChange(e.target.value)}
        >
          <option value="">Select a team…</option>
          {teams?.map((t) => (
            <option key={t.id} value={t.id}>{t.name}</option>
          ))}
        </select>
        {errors.team_id && <p className="text-xs text-[hsl(var(--destructive))]">{errors.team_id.message}</p>}
      </div>

      {/* Title */}
      <div className="space-y-1.5">
        <Label>Title</Label>
        <Input placeholder="e.g. Dinner at Commons" {...register("title")} />
        {errors.title && <p className="text-xs text-[hsl(var(--destructive))]">{errors.title.message}</p>}
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
            {CURRENCIES.map(({ code, label }) => (
              <option key={code} value={code}>{label}</option>
            ))}
          </select>
        </div>
        <div className="space-y-1.5">
          <Label>Date</Label>
          <Input type="date" {...register("expense_date")} />
          {errors.expense_date && <p className="text-xs text-[hsl(var(--destructive))]">{errors.expense_date.message}</p>}
        </div>
      </div>

      {/* Amount */}
      <div className="space-y-1.5">
        <Label>Amount</Label>
        <AmountInput
          value={amount}
          currency={currency}
          onChange={(v) => { setAmount(v); setValue("amount", v); }}
        />
        {errors.amount && <p className="text-xs text-[hsl(var(--destructive))]">{errors.amount.message}</p>}
      </div>

      {/* Paid by */}
      {selectedTeamId && teamMembers && teamMembers.length > 0 && (
        <div className="space-y-1.5">
          <Label>Paid by</Label>
          <select
            className={SELECT_CLASS}
            {...register("paid_by")}
            defaultValue={me?.id ?? ""}
          >
            <option value="">Select who paid…</option>
            {teamMembers.map((m) => (
              <option key={m.user_id} value={m.user_id}>
                {m.display_name}{isAnonymousMember(m) ? " (anon)" : ""}
              </option>
            ))}
          </select>
          {errors.paid_by && <p className="text-xs text-[hsl(var(--destructive))]">{errors.paid_by.message}</p>}
        </div>
      )}

      {/* Split method */}
      <div className="space-y-1.5">
        <Label>Split method</Label>
        <select
          className={SELECT_CLASS}
          {...register("split_method")}
          onChange={(e) => { setValue("split_method", e.target.value as SplitMethod); setSplitMethod(e.target.value as SplitMethod); }}
        >
          {SPLIT_METHODS.map(({ value, label }) => (
            <option key={value} value={value}>{label}</option>
          ))}
        </select>
      </div>

      {/* Participants */}
      {selectedTeamId && (
        <div className="space-y-2">
          <Label>Participants</Label>
          <MemberPicker
            teamId={selectedTeamId}
            selected={participants}
            onChange={setParticipants}
            onMembersChange={setParticipantObjects}
          />
        </div>
      )}

      {/* SplitBuilder */}
      {participants.length > 0 && amount > 0 && (
        <div className="space-y-2 border rounded-xl p-3 bg-[hsl(var(--muted)/0.4)]">
          <Label className="text-xs uppercase tracking-wide text-[hsl(var(--muted-foreground))]">Split preview</Label>
          <SplitBuilder
            participants={participantObjects}
            total={amount}
            currency={currency}
            method={splitMethod}
            value={splits}
            onChange={setSplits}
          />
        </div>
      )}

      {/* Note */}
      <div className="space-y-1.5">
        <Label>Note <span className="text-[hsl(var(--muted-foreground))]">(optional)</span></Label>
        <Input placeholder="Any additional details" {...register("note")} />
      </div>

      <div className="flex gap-3 pt-2">
        <Button type="submit" disabled={isPending || !selectedTeamId} className="flex-1">
          {isPending ? "Adding…" : "Add expense"}
        </Button>
        <SheetClose asChild>
          <Button type="button" variant="outline">Cancel</Button>
        </SheetClose>
      </div>
    </form>
  );
}

type Props = {
  children: React.ReactNode;
};

export function AddExpenseSheet({ children }: Props) {
  const [open, setOpen] = useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>{children}</SheetTrigger>
      <SheetContent title="Add expense" description="Record a new team expense">
        <AddExpenseForm onSuccess={() => setOpen(false)} />
      </SheetContent>
    </Sheet>
  );
}
