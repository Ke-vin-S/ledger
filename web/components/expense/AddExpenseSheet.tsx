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
import { useTeams } from "@/hooks/useTeam";
import { useCreateExpense } from "@/hooks/useExpenses";
import { ApiRequestError } from "@/lib/api";

const SPLIT_METHODS = ["equal", "exact", "percentage", "shares"] as const;

const CURRENCIES = [
  { code: "LKR", label: "LKR — Sri Lanka Rupee" },
  { code: "USD", label: "USD — US Dollar" },
  { code: "EUR", label: "EUR — Euro" },
  { code: "GBP", label: "GBP — British Pound" },
  { code: "INR", label: "INR — Indian Rupee" },
  { code: "SGD", label: "SGD — Singapore Dollar" },
  { code: "AUD", label: "AUD — Australian Dollar" },
];

const schema = z.object({
  team_id: z.string().min(1, "Select a team"),
  title: z.string().min(1, "Title is required"),
  amount: z.number().int().positive("Amount must be positive"),
  currency: z.string().min(1, "Currency is required"),
  split_method: z.enum(SPLIT_METHODS),
  expense_date: z.string().min(1, "Date is required"),
  note: z.string().optional(),
});
type FormValues = z.infer<typeof schema>;

function AddExpenseForm({ onSuccess }: { onSuccess: () => void }) {
  const { data: teams } = useTeams();
  const [selectedTeamId, setSelectedTeamId] = useState<string>("");
  const [amount, setAmount] = useState(0);
  const [currency, setCurrency] = useState("LKR");
  const [serverError, setServerError] = useState<string | null>(null);

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

  async function onSubmit(data: FormValues) {
    if (!data.team_id) return;
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
      reset();
      setAmount(0);
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

      <div className="space-y-1.5">
        <Label>Team</Label>
        <select
          className="w-full h-9 rounded-md border border-[hsl(var(--input))] bg-[hsl(var(--background))] px-3 text-sm focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]"
          {...register("team_id")}
          onChange={(e) => {
            setValue("team_id", e.target.value);
            setSelectedTeamId(e.target.value);
          }}
        >
          <option value="">Select a team…</option>
          {teams?.map((t) => (
            <option key={t.id} value={t.id}>{t.name}</option>
          ))}
        </select>
        {errors.team_id && (
          <p className="text-xs text-[hsl(var(--destructive))]">{errors.team_id.message}</p>
        )}
      </div>

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
            {CURRENCIES.map(({ code, label }) => (
              <option key={code} value={code}>{label}</option>
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
