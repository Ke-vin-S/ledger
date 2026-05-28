"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import Link from "next/link";
import { useTeams, useCreateTeam } from "@/hooks/useTeam";
import { useMe } from "@/hooks/useAuth";
import { Skeleton } from "@/components/shared/Skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ArrowRight, Plus } from "lucide-react";
import { ApiRequestError } from "@/lib/api";

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
  name: z.string().min(1, "Name is required").max(80),
  description: z.string().max(200).optional(),
  currency: z.string().min(1, "Currency is required"),
});
type FormValues = z.infer<typeof schema>;

function CreateTeamForm({ defaultCurrency, onClose }: { defaultCurrency: string; onClose: () => void }) {
  const { mutateAsync, isPending } = useCreateTeam();
  const [serverError, setServerError] = useState<string | null>(null);

  const { register, handleSubmit, formState: { errors } } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { currency: defaultCurrency },
  });

  async function onSubmit(data: FormValues) {
    setServerError(null);
    try {
      await mutateAsync({ name: data.name, description: data.description, currency: data.currency });
      onClose();
    } catch (err) {
      if (err instanceof ApiRequestError) setServerError(err.error.message);
      else setServerError("Failed to create team.");
    }
  }

  return (
    <Card className="mb-6">
      <CardHeader>
        <CardTitle className="text-base">New team</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {serverError && (
            <p className="text-sm text-[hsl(var(--destructive))]">{serverError}</p>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="name">Name</Label>
            <Input id="name" placeholder="e.g. Roommates" {...register("name")} />
            {errors.name && (
              <p className="text-xs text-[hsl(var(--destructive))]">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="description">
              Description <span className="text-[hsl(var(--muted-foreground))]">(optional)</span>
            </Label>
            <Input id="description" placeholder="What's this team for?" {...register("description")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="currency">Currency</Label>
            <select
              id="currency"
              className="w-full h-9 rounded-md border border-[hsl(var(--input))] bg-[hsl(var(--background))] px-3 text-sm focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]"
              {...register("currency")}
            >
              {CURRENCIES.map(({ code, label }) => (
                <option key={code} value={code}>{label}</option>
              ))}
            </select>
            {errors.currency && (
              <p className="text-xs text-[hsl(var(--destructive))]">{errors.currency.message}</p>
            )}
            <p className="text-xs text-[hsl(var(--muted-foreground))]">
              All expenses in this team will default to this currency.
            </p>
          </div>

          <div className="flex gap-2 pt-1">
            <Button type="submit" disabled={isPending} size="sm">
              {isPending ? "Creating…" : "Create team"}
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

export default function TeamsPage() {
  const { data: teams, isLoading } = useTeams();
  const { data: me } = useMe();
  const [showCreate, setShowCreate] = useState(false);

  const defaultCurrency = me?.currency_pref ?? "LKR";

  return (
    <div className="p-8 space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <p className="text-sm text-[hsl(var(--muted-foreground))]">Manage your expense-sharing groups</p>
        {!showCreate && (
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-1" /> New team
          </Button>
        )}
      </div>

      {showCreate && (
        <CreateTeamForm defaultCurrency={defaultCurrency} onClose={() => setShowCreate(false)} />
      )}

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : !teams?.length ? (
        <p className="text-sm text-[hsl(var(--muted-foreground))]">
          No teams yet. Create one to get started.
        </p>
      ) : (
        <div className="space-y-2">
          {teams.map((team) => (
            <Link
              key={team.id}
              href={`/teams/${team.id}` as never}
              className="flex items-center justify-between p-4 rounded-xl border bg-[hsl(var(--card))] hover:bg-[hsl(var(--muted))] transition-colors"
            >
              <div>
                <p className="font-medium text-sm">{team.name}</p>
                <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
                  {team.currency}
                  {team.description && ` · ${team.description}`}
                </p>
              </div>
              <ArrowRight className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
