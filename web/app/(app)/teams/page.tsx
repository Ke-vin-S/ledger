"use client";

import { useId, useState } from "react";
import Link from "next/link";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowRight, Plus } from "lucide-react";
import { useTeams, useCreateTeam } from "@/hooks/useTeam";
import { useMe } from "@/hooks/useAuth";
import { ApiRequestError } from "@/lib/api";
import { CURRENCIES } from "@/constants/config";
import { ROUTES } from "@/constants/routes";
import { PageHeader } from "@/components/shared/PageHeader";
import { QueryBoundary } from "@/components/query-boundary";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/components/ui/toast";

const schema = z.object({
  name: z.string().min(1, "Name is required").max(80),
  description: z.string().max(200).optional(),
  currency: z.string().min(1, "Currency is required"),
});
type FormValues = z.infer<typeof schema>;

function CreateTeamForm({
  defaultCurrency,
  onClose,
}: {
  defaultCurrency: string;
  onClose: () => void;
}) {
  const { mutateAsync, isPending } = useCreateTeam();
  const { toast } = useToast();
  const formId = useId();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { currency: defaultCurrency },
  });

  async function onSubmit(data: FormValues) {
    setServerError(null);
    try {
      await mutateAsync({
        name: data.name,
        description: data.description,
        currency: data.currency,
      });
      toast({
        title: "Team created",
        description: `${data.name} is ready for shared expenses.`,
        variant: "success",
      });
      onClose();
    } catch (err) {
      setServerError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to create team.",
      );
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      {serverError ? (
        <p
          role="alert"
          className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
        >
          {serverError}
        </p>
      ) : null}
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-name`}>Name</Label>
        <Input
          id={`${formId}-name`}
          placeholder="e.g. Roommates"
          aria-invalid={errors.name ? true : undefined}
          {...register("name")}
        />
        {errors.name ? (
          <p className="text-xs text-destructive">{errors.name.message}</p>
        ) : null}
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-description`}>
          Description <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id={`${formId}-description`}
          placeholder="What is this team for?"
          {...register("description")}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-currency`}>Currency</Label>
        <Controller
          name="currency"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id={`${formId}-currency`}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CURRENCIES.map(({ code, label }) => (
                  <SelectItem key={code} value={code}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <p className="text-xs text-muted-foreground">
          All expenses in this team will default to this currency.
        </p>
      </div>
      <DialogFooter>
        <Button type="submit" disabled={isPending}>
          {isPending ? "Creating…" : "Create team"}
        </Button>
        <DialogClose asChild>
          <Button type="button" variant="outline">
            Cancel
          </Button>
        </DialogClose>
      </DialogFooter>
    </form>
  );
}

function TeamCard({
  team,
}: {
  team: { id: string; name: string; description?: string; currency: string };
}) {
  return (
    <Link
      href={ROUTES.team(team.id) as never}
      className="group flex min-h-44 flex-col justify-between rounded-xl border bg-card p-5 transition-colors hover:border-primary/50 hover:bg-muted/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h2 className="truncate text-lg font-semibold">{team.name}</h2>
          <p className="mt-2 line-clamp-2 text-sm leading-6 text-muted-foreground">
            {team.description || "A shared space for expenses and settlements."}
          </p>
        </div>
        <ArrowRight
          className="h-5 w-5 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-1"
          aria-hidden="true"
        />
      </div>
      <div className="mt-6 flex items-center justify-between gap-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        <span>{team.currency}</span>
        <span>Open team</span>
      </div>
    </Link>
  );
}

export default function TeamsPage() {
  const teamsQuery = useTeams();
  const { data: me } = useMe();
  const [dialogOpen, setDialogOpen] = useState(false);
  const defaultCurrency = me?.currency_pref ?? "LKR";

  return (
    <main className="mx-auto max-w-6xl space-y-8 p-4 md:p-8">
      <PageHeader
        eyebrow="Shared spaces"
        title="Your teams"
        description="Keep group expenses, balances, and activity together without losing the thread."
        action={
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4" aria-hidden="true" /> New team
              </Button>
            </DialogTrigger>
            <DialogContent
              title="Create a team"
              description="Set the shared currency and give your group a home."
            >
              <CreateTeamForm
                defaultCurrency={defaultCurrency}
                onClose={() => setDialogOpen(false)}
              />
            </DialogContent>
          </Dialog>
        }
      />

      <QueryBoundary
        {...teamsQuery}
        isEmpty={(teams) => teams.length === 0}
        emptyMessage="Create your first team to start sharing expenses with your people."
        className="min-h-64"
      >
        {(teams) => (
          <div className="grid gap-4 md:grid-cols-2">
            {teams.map((team) => (
              <TeamCard key={team.id} team={team} />
            ))}
          </div>
        )}
      </QueryBoundary>
    </main>
  );
}
