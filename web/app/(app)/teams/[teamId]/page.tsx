"use client";

import { useId, useState } from "react";
import { useParams } from "next/navigation";
import {
  Check,
  Link2,
  Mail,
  Plus,
  RefreshCw,
  UserPlus,
  UserX,
  X,
} from "lucide-react";
import {
  isAnonymousMember,
  useAddAnonymousMember,
  useCancelInvitation,
  useGenerateClaimToken,
  useInviteMember,
  useResendInvitation,
  useTeam,
  useTeamInvitations,
  useTeamMembers,
} from "@/hooks/useTeam";
import { useExpenses } from "@/hooks/useExpenses";
import { useTeamBalances } from "@/hooks/useSettlements";
import { ActivityFeed } from "@/components/team/ActivityFeed";
import { ExpenseCard } from "@/components/expense/ExpenseCard";
import { AddExpenseSheet } from "@/components/expense/AddExpenseSheet";
import { DebtBar } from "@/components/settlement/DebtBar";
import { Avatar } from "@/components/shared/Avatar";
import { QueryBoundary } from "@/components/query-boundary";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useToast } from "@/components/ui/toast";
import { ApiRequestError } from "@/lib/api";
import { formatDate } from "@/lib/utils";

type TeamTab = "expenses" | "members" | "balances" | "activity";

function isTeamTab(value: string): value is TeamTab {
  return (
    value === "expenses" ||
    value === "members" ||
    value === "balances" ||
    value === "activity"
  );
}

function InviteMemberDialog({ teamId }: { teamId: string }) {
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const { mutateAsync, isPending } = useInviteMember(teamId);
  const { toast } = useToast();
  const emailId = useId();

  async function submit() {
    if (!email.trim()) {
      setError("Email is required");
      return;
    }
    setError("");
    try {
      await mutateAsync({ email: email.trim() });
      toast({
        title: "Invitation sent",
        description: email.trim(),
        variant: "success",
      });
      setEmail("");
      setOpen(false);
    } catch (err) {
      setError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to send invitation.",
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <UserPlus className="h-4 w-4" aria-hidden="true" /> Invite by email
        </Button>
      </DialogTrigger>
      <DialogContent
        title="Invite by email"
        description="Send a secure invitation to join this team."
      >
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="space-y-4"
        >
          <div className="space-y-1.5">
            <Label htmlFor={emailId}>Email address</Label>
            <Input
              id={emailId}
              type="email"
              autoComplete="email"
              placeholder="colleague@example.com"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              aria-invalid={error ? true : undefined}
            />
          </div>
          {error ? (
            <p
              role="alert"
              className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
            >
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="submit" disabled={isPending}>
              {isPending ? "Sending…" : "Send invitation"}
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
  );
}

function AnonymousMemberDialog({ teamId }: { teamId: string }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const { mutateAsync, isPending } = useAddAnonymousMember(teamId);
  const { toast } = useToast();
  const nameId = useId();

  async function submit() {
    if (!name.trim()) {
      setError("Name is required");
      return;
    }
    setError("");
    try {
      await mutateAsync({ display_name: name.trim() });
      toast({
        title: "Member added",
        description: `${name.trim()} can claim their account later.`,
        variant: "success",
      });
      setName("");
      setOpen(false);
    } catch (err) {
      setError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to add member.",
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <UserX className="h-4 w-4" aria-hidden="true" /> Add without account
        </Button>
      </DialogTrigger>
      <DialogContent
        title="Add without an account"
        description="Create a provisional member who can claim their identity later."
      >
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="space-y-4"
        >
          <div className="space-y-1.5">
            <Label htmlFor={nameId}>Display name</Label>
            <Input
              id={nameId}
              placeholder="e.g. Rahul"
              value={name}
              onChange={(event) => setName(event.target.value)}
              aria-invalid={error ? true : undefined}
            />
          </div>
          {error ? (
            <p
              role="alert"
              className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
            >
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="submit" disabled={isPending}>
              {isPending ? "Adding…" : "Add member"}
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
  );
}

function PendingInvitations({ teamId }: { teamId: string }) {
  const query = useTeamInvitations(teamId);
  const cancelInvitation = useCancelInvitation(teamId);
  const resendInvitation = useResendInvitation(teamId);
  const { toast } = useToast();
  const [busyId, setBusyId] = useState<string | null>(null);

  async function cancel(id: string, email: string) {
    setBusyId(id);
    try {
      await cancelInvitation.mutateAsync(id);
      toast({ title: "Invitation cancelled", description: email });
    } catch (err) {
      toast({
        title: "Could not cancel invitation",
        description:
          err instanceof ApiRequestError ? err.error.message : "Try again.",
        variant: "destructive",
      });
    } finally {
      setBusyId(null);
    }
  }

  async function resend(id: string, email: string) {
    setBusyId(id);
    try {
      await resendInvitation.mutateAsync(id);
      toast({
        title: "Invitation resent",
        description: email,
        variant: "success",
      });
    } catch (err) {
      toast({
        title: "Could not resend invitation",
        description:
          err instanceof ApiRequestError ? err.error.message : "Try again.",
        variant: "destructive",
      });
    } finally {
      setBusyId(null);
    }
  }

  return (
    <QueryBoundary
      {...query}
      isEmpty={() => false}
      className="min-h-0 border-0 bg-transparent p-0 shadow-none"
    >
      {(invitations) =>
        invitations.length === 0 ? null : (
          <section
            aria-labelledby="pending-invitations"
            className="space-y-3 border-t pt-6"
          >
            <h3 id="pending-invitations" className="text-sm font-semibold">
              Pending invitations
            </h3>
            <div className="space-y-2">
              {invitations.map((invitation) => (
                <div
                  key={invitation.id}
                  className="flex flex-wrap items-center gap-3 rounded-xl border border-dashed bg-card p-4"
                >
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
                    <Mail className="h-4 w-4" aria-hidden="true" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">
                      {invitation.email}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      Expires {formatDate(invitation.expires_at)}
                    </p>
                  </div>
                  <Badge variant="outline">Pending</Badge>
                  <div className="flex gap-1">
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      disabled={busyId === invitation.id}
                      aria-label={`Resend invitation to ${invitation.email}`}
                      onClick={() =>
                        void resend(invitation.id, invitation.email)
                      }
                    >
                      <RefreshCw className="h-4 w-4" aria-hidden="true" />
                    </Button>
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      disabled={busyId === invitation.id}
                      aria-label={`Cancel invitation to ${invitation.email}`}
                      onClick={() =>
                        void cancel(invitation.id, invitation.email)
                      }
                    >
                      <X className="h-4 w-4" aria-hidden="true" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )
      }
    </QueryBoundary>
  );
}

function MembersTab({ teamId }: { teamId: string }) {
  const membersQuery = useTeamMembers(teamId);
  const generateClaimToken = useGenerateClaimToken();
  const { toast } = useToast();
  const [copiedId, setCopiedId] = useState<string | null>(null);

  async function copyClaimLink(userId: string, name: string) {
    try {
      const result = await generateClaimToken.mutateAsync(userId);
      await navigator.clipboard.writeText(result.claim_url);
      setCopiedId(userId);
      toast({
        title: "Claim link copied",
        description: `Share it with ${name}.`,
        variant: "success",
      });
      window.setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      toast({
        title: "Could not copy claim link",
        description:
          err instanceof ApiRequestError ? err.error.message : "Try again.",
        variant: "destructive",
      });
    }
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap gap-2">
        <InviteMemberDialog teamId={teamId} />
        <AnonymousMemberDialog teamId={teamId} />
      </div>
      <QueryBoundary
        {...membersQuery}
        isEmpty={(members) => members.length === 0}
        emptyMessage="No members are visible in this team yet."
        className="min-h-48"
      >
        {(members) => (
          <div className="grid gap-3 sm:grid-cols-2">
            {members.map((member) => {
              const anonymous = isAnonymousMember(member);
              return (
                <div
                  key={member.user_id}
                  className="flex items-center gap-3 rounded-xl border bg-card p-4"
                >
                  <Avatar name={member.display_name} size="md" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="truncate text-sm font-semibold">
                        {member.display_name}
                      </p>
                      {anonymous ? (
                        <Badge variant="outline">Anonymous</Badge>
                      ) : null}
                    </div>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {member.role} · {member.status}
                    </p>
                  </div>
                  {anonymous ? (
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      aria-label={`Copy claim link for ${member.display_name}`}
                      onClick={() =>
                        void copyClaimLink(member.user_id, member.display_name)
                      }
                    >
                      {copiedId === member.user_id ? (
                        <Check
                          className="h-4 w-4 text-positive"
                          aria-hidden="true"
                        />
                      ) : (
                        <Link2 className="h-4 w-4" aria-hidden="true" />
                      )}
                    </Button>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </QueryBoundary>
      <PendingInvitations teamId={teamId} />
    </div>
  );
}

export default function TeamPage() {
  const { teamId } = useParams<{ teamId: string }>();
  const teamQuery = useTeam(teamId);
  const membersQuery = useTeamMembers(teamId);
  const expensesQuery = useExpenses(teamId);
  const balancesQuery = useTeamBalances(teamId);
  const [tab, setTab] = useState<TeamTab>("expenses");

  return (
    <main className="mx-auto max-w-6xl space-y-8 p-4 md:p-8">
      <QueryBoundary {...teamQuery} isEmpty={() => false} className="min-h-64">
        {(team) => {
          const members = membersQuery.data ?? [];
          return (
            <>
              <Card className="overflow-hidden border-primary/20 bg-card">
                <CardContent className="flex flex-col gap-6 p-6 md:flex-row md:items-end md:justify-between md:p-8">
                  <div className="min-w-0">
                    <div className="mb-3 flex flex-wrap items-center gap-2">
                      <Badge variant="secondary">{team.currency}</Badge>
                      <span className="text-xs text-muted-foreground">
                        {members.length}{" "}
                        {members.length === 1 ? "member" : "members"}
                      </span>
                    </div>
                    <h1 className="font-display text-3xl font-semibold tracking-tight md:text-4xl">
                      {team.name}
                    </h1>
                    <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
                      {team.description ||
                        "Shared expenses, balances, and team activity in one calm workspace."}
                    </p>
                  </div>
                  <div
                    className="flex -space-x-2"
                    aria-label={`${members.length} team members`}
                  >
                    {members.slice(0, 5).map((member) => (
                      <Avatar
                        key={member.user_id}
                        name={member.display_name}
                        size="md"
                        className="ring-2 ring-card"
                      />
                    ))}
                  </div>
                </CardContent>
              </Card>

              <Tabs
                value={tab}
                onValueChange={(value) => {
                  if (isTeamTab(value)) setTab(value);
                }}
              >
                <TabsList
                  aria-label={`${team.name} sections`}
                  className="w-full justify-start overflow-x-auto sm:w-auto"
                >
                  <TabsTrigger value="expenses">
                    Expenses
                    {expensesQuery.data
                      ? ` (${expensesQuery.data.length})`
                      : ""}
                  </TabsTrigger>
                  <TabsTrigger value="members">
                    Members{members.length ? ` (${members.length})` : ""}
                  </TabsTrigger>
                  <TabsTrigger value="balances">
                    Balances
                    {balancesQuery.data
                      ? ` (${balancesQuery.data.length})`
                      : ""}
                  </TabsTrigger>
                  <TabsTrigger value="activity">Activity</TabsTrigger>
                </TabsList>

                <TabsContent value="expenses" className="mt-6">
                  <div className="mb-5 flex justify-end">
                    <AddExpenseSheet>
                      <Button>
                        <Plus className="h-4 w-4" aria-hidden="true" /> Add
                        expense
                      </Button>
                    </AddExpenseSheet>
                  </div>
                  <QueryBoundary
                    {...expensesQuery}
                    isEmpty={(expenses) => expenses.length === 0}
                    emptyMessage="No expenses yet. Add the first shared expense to get started."
                    className="min-h-48"
                  >
                    {(expenses) => (
                      <div className="grid gap-3">
                        {expenses.map((expense) => (
                          <ExpenseCard
                            key={expense.id}
                            expense={expense}
                            teamId={teamId}
                          />
                        ))}
                      </div>
                    )}
                  </QueryBoundary>
                </TabsContent>

                <TabsContent value="members" className="mt-6">
                  <MembersTab teamId={teamId} />
                </TabsContent>

                <TabsContent value="balances" className="mt-6">
                  <QueryBoundary
                    {...balancesQuery}
                    isEmpty={(balances) => balances.length === 0}
                    emptyMessage="All settled up. No outstanding team balances."
                    className="min-h-48"
                  >
                    {(balances) => (
                      <div className="grid gap-3">
                        {balances.map((balance) => (
                          <DebtBar
                            key={balance.counterparty_id}
                            counterpartyName={balance.counterparty_name}
                            netAmount={balance.net_amount}
                          />
                        ))}
                      </div>
                    )}
                  </QueryBoundary>
                </TabsContent>

                <TabsContent value="activity" className="mt-6">
                  <ActivityFeed teamId={teamId} />
                </TabsContent>
              </Tabs>
            </>
          );
        }}
      </QueryBoundary>
    </main>
  );
}
