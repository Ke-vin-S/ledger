"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  useTeam,
  useTeamMembers,
  useInviteMember,
  useAddAnonymousMember,
  useGenerateClaimToken,
  useRemoveMember,
  isAnonymousMember,
} from "@/hooks/useTeam";
import { useMe } from "@/hooks/useAuth";
import { useExpenses, useCreateExpense, useVoidExpense } from "@/hooks/useExpenses";
import { useTeamBalances } from "@/hooks/useSettlements";
import { ActivityFeed } from "@/components/team/ActivityFeed";
import { ExpenseCard } from "@/components/expense/ExpenseCard";
import { AmountInput } from "@/components/expense/AmountInput";
import { MemberPicker } from "@/components/expense/MemberPicker";
import { SplitBuilder } from "@/components/expense/SplitBuilder";
import type { PickedMember } from "@/types/team.types";
import type { SplitMethod, SplitEntry } from "@/types/expense.types";
import { DebtBar } from "@/components/settlement/DebtBar";
import { Skeleton } from "@/components/shared/Skeleton";
import { Avatar } from "@/components/shared/Avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Plus, UserPlus, UserX, Link2, Check } from "lucide-react";
import { ApiRequestError } from "@/lib/api";

import { CURRENCY_CODES, SPLIT_METHODS } from "@/constants/config";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const expenseSchema = z.object({
  title: z.string().min(1, "Title is required"),
  amount: z.number().int().positive("Amount must be positive"),
  currency: z.string().min(1),
  split_method: z.enum(["equal", "exact", "percentage", "shares"]),
  expense_date: z.string().min(1, "Date is required"),
  paid_by: z.string().min(1, "Select who paid"),
  note: z.string().optional(),
});
type ExpenseFormValues = z.infer<typeof expenseSchema>;

function CreateExpenseForm({ teamId, onClose }: { teamId: string; onClose: () => void }) {
  const { data: me } = useMe();
  const { data: teamMembers } = useTeamMembers(teamId);
  const { mutateAsync, isPending } = useCreateExpense(teamId);
  const [serverError, setServerError] = useState<string | null>(null);
  const [amount, setAmount] = useState(0);
  const [currency, setCurrency] = useState("LKR");
  const [splitMethod, setSplitMethod] = useState<SplitMethod>("equal");
  const [participants, setParticipants] = useState<string[]>([]);
  const [participantObjects, setParticipantObjects] = useState<PickedMember[]>([]);
  const [splits, setSplits] = useState<SplitEntry[]>([]);

  const { register, handleSubmit, setValue, control, formState: { errors } } = useForm<ExpenseFormValues>({
    resolver: zodResolver(expenseSchema),
    defaultValues: {
      currency: "LKR",
      split_method: "equal",
      expense_date: new Date().toISOString().split("T")[0],
      paid_by: me?.id ?? "",
    },
  });

  async function onSubmit(data: ExpenseFormValues) {
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
            {errors.title && <p className="text-xs text-[hsl(var(--destructive))]">{errors.title.message}</p>}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Currency</Label>
              <Controller
                name="currency"
                control={control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={(v) => { field.onChange(v); setCurrency(v); }}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {CURRENCY_CODES.map((c) => <SelectItem key={c} value={c}>{c}</SelectItem>)}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Date</Label>
              <Input type="date" {...register("expense_date")} />
              {errors.expense_date && <p className="text-xs text-[hsl(var(--destructive))]">{errors.expense_date.message}</p>}
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Amount</Label>
            <AmountInput value={amount} currency={currency} onChange={(v) => { setAmount(v); setValue("amount", v); }} />
            {errors.amount && <p className="text-xs text-[hsl(var(--destructive))]">{errors.amount.message}</p>}
          </div>

          {/* Paid by */}
          {teamMembers && teamMembers.length > 0 && (
            <div className="space-y-1.5">
              <Label>Paid by</Label>
              <Controller
                name="paid_by"
                control={control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select who paid…" />
                    </SelectTrigger>
                    <SelectContent>
                      {teamMembers.map((m) => (
                        <SelectItem key={m.user_id} value={m.user_id}>
                          {m.display_name}{isAnonymousMember(m) ? " (anon)" : ""}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {errors.paid_by && <p className="text-xs text-[hsl(var(--destructive))]">{errors.paid_by.message}</p>}
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Split method</Label>
              <Controller
                name="split_method"
                control={control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={(v) => { field.onChange(v); setSplitMethod(v as SplitMethod); }}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {SPLIT_METHODS.map(({ value, label }) => (
                        <SelectItem key={value} value={value}>{label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Note <span className="text-[hsl(var(--muted-foreground))]">(opt.)</span></Label>
              <Input placeholder="Any details" {...register("note")} />
            </div>
          </div>

          {/* Participants */}
          <div className="space-y-1.5">
            <Label>Participants</Label>
            <MemberPicker
              teamId={teamId}
              selected={participants}
              onChange={setParticipants}
              onMembersChange={setParticipantObjects}
            />
          </div>

          {/* Split preview */}
          {participants.length > 0 && amount > 0 && (
            <div className="border rounded-xl p-3 bg-[hsl(var(--muted)/0.4)] space-y-2">
              <p className="text-[0.7rem] uppercase tracking-wide font-semibold text-[hsl(var(--muted-foreground))]">Split preview</p>
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

// ── Members Tab ─────────────────────────────────────────────────────────────

function MembersTab({ teamId }: { teamId: string }) {
  const { data: members } = useTeamMembers(teamId);
  const { mutateAsync: inviteMember } = useInviteMember(teamId);
  const { mutateAsync: addAnonymous } = useAddAnonymousMember(teamId);
  const { mutateAsync: generateClaimToken } = useGenerateClaimToken();
  useRemoveMember(teamId);

  const [showInvite, setShowInvite] = useState(false);
  const [showAddAnon, setShowAddAnon] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteError, setInviteError] = useState("");
  const [invitePending, setInvitePending] = useState(false);
  const [anonName, setAnonName] = useState("");
  const [anonError, setAnonError] = useState("");
  const [anonPending, setAnonPending] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  async function handleInvite() {
    if (!inviteEmail) { setInviteError("Email is required"); return; }
    setInviteError(""); setInvitePending(true);
    try {
      await inviteMember({ email: inviteEmail });
      setInviteEmail(""); setShowInvite(false);
    } catch (err) {
      setInviteError(err instanceof ApiRequestError ? err.error.message : "Failed to invite");
    } finally { setInvitePending(false); }
  }

  async function handleAddAnon() {
    if (!anonName.trim()) { setAnonError("Name is required"); return; }
    setAnonError(""); setAnonPending(true);
    try {
      await addAnonymous({ display_name: anonName.trim() });
      setAnonName(""); setShowAddAnon(false);
    } catch (err) {
      setAnonError(err instanceof ApiRequestError ? err.error.message : "Failed to add");
    } finally { setAnonPending(false); }
  }

  async function handleCopyClaimLink(userId: string) {
    try {
      const res = await generateClaimToken(userId);
      await navigator.clipboard.writeText(res.claim_url);
      setCopiedId(userId);
      setTimeout(() => setCopiedId(null), 2000);
    } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      {/* Action buttons */}
      <div className="flex gap-2 flex-wrap">
        <Button size="sm" variant="outline" onClick={() => { setShowInvite(!showInvite); setShowAddAnon(false); }}>
          <UserPlus className="h-3.5 w-3.5 mr-1.5" />
          Invite by email
        </Button>
        <Button size="sm" variant="outline" onClick={() => { setShowAddAnon(!showAddAnon); setShowInvite(false); }}>
          <UserX className="h-3.5 w-3.5 mr-1.5" />
          Add without account
        </Button>
      </div>

      {/* Invite by email form */}
      {showInvite && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">Email</Label>
              <Input
                type="email"
                placeholder="colleague@example.com"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), handleInvite())}
                className="h-8 text-sm"
              />
            </div>
            {inviteError && <p className="text-xs text-[hsl(var(--destructive))]">{inviteError}</p>}
            <div className="flex gap-2">
              <Button size="sm" onClick={handleInvite} disabled={invitePending}>
                {invitePending ? "Sending…" : "Send invite"}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => { setShowInvite(false); setInviteError(""); }}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Add anonymous form */}
      {showAddAnon && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">Name</Label>
              <Input
                placeholder="e.g. Rahul (no account)"
                value={anonName}
                onChange={(e) => setAnonName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), handleAddAnon())}
                className="h-8 text-sm"
              />
            </div>
            {anonError && <p className="text-xs text-[hsl(var(--destructive))]">{anonError}</p>}
            <div className="flex gap-2">
              <Button size="sm" onClick={handleAddAnon} disabled={anonPending}>
                {anonPending ? "Adding…" : "Add member"}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => { setShowAddAnon(false); setAnonError(""); }}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Members list */}
      <div className="space-y-2">
        {members?.map((m) => {
          const isAnon = isAnonymousMember(m);
          return (
            <div key={m.user_id} className="flex items-center gap-3 p-3 border rounded-xl bg-[hsl(var(--card))]">
              <Avatar name={m.display_name} size="sm" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <p className="text-sm font-medium truncate">{m.display_name}</p>
                  {isAnon && <Badge variant="outline" className="text-[0.65rem] py-0 px-1.5 border-dashed">anon</Badge>}
                </div>
                <p className="text-xs text-[hsl(var(--muted-foreground))]">
                  {isAnon ? "No account — awaiting claim" : m.status}
                </p>
              </div>
              <div className="flex items-center gap-1.5">
                <Badge variant="secondary" className="text-xs">{m.role}</Badge>
                {isAnon && (
                  <button
                    onClick={() => handleCopyClaimLink(m.user_id)}
                    className="p-1.5 rounded-md hover:bg-[hsl(var(--muted))] transition-colors text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
                    title="Copy claim link"
                  >
                    {copiedId === m.user_id ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Link2 className="h-3.5 w-3.5" />}
                  </button>
                )}
              </div>
            </div>
          );
        })}
        {!members?.length && (
          <p className="text-sm text-[hsl(var(--muted-foreground))]">No members found.</p>
        )}
      </div>
    </div>
  );
}

// ── Main Page ────────────────────────────────────────────────────────────────

export default function TeamPage() {
  const { teamId } = useParams<{ teamId: string }>();
  const { data: team, isLoading: teamLoading } = useTeam(teamId);
  const { data: expenses, isLoading: expensesLoading } = useExpenses(teamId);
  const { data: balances } = useTeamBalances(teamId);
  useVoidExpense(teamId);
  const [showCreateExpense, setShowCreateExpense] = useState(false);
  const [activeTab, setActiveTab] = useState<"expenses" | "members" | "balances" | "activity">("expenses");

  if (teamLoading) {
    return (
      <div className="p-4 md:p-8 space-y-4">
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
    <div className="p-4 md:p-8 space-y-6 max-w-3xl">
      <div>
        <h1 className="text-3xl font-bold">{team.name}</h1>
        {team.description && (
          <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">{team.description}</p>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b overflow-x-auto scrollbar-none">
        {tabs.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap flex-shrink-0 ${
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
              {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-16" />)}
            </div>
          ) : !expenses?.length ? (
            <p className="text-sm text-[hsl(var(--muted-foreground))]">No expenses yet.</p>
          ) : (
            <div className="space-y-2">
              {expenses.map((expense) => (
                <ExpenseCard
                  key={expense.id}
                  expense={expense}
                  teamId={teamId}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {/* Members tab */}
      {activeTab === "members" && <MembersTab teamId={teamId} />}

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
