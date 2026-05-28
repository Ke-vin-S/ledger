"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import Link from "next/link";
import { useTeams, useCreateTeam } from "@/hooks/useTeam";
import { Skeleton } from "@/components/shared/Skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ArrowRight, Plus } from "lucide-react";
import { ApiRequestError } from "@/lib/api";

const schema = z.object({
  name: z.string().min(1, "Name is required").max(80),
  description: z.string().max(200).optional(),
});
type FormValues = z.infer<typeof schema>;

function CreateTeamForm({ onClose }: { onClose: () => void }) {
  const { mutateAsync, isPending } = useCreateTeam();
  const [serverError, setServerError] = useState<string | null>(null);

  const { register, handleSubmit, formState: { errors } } = useForm<FormValues>({
    resolver: zodResolver(schema),
  });

  async function onSubmit(data: FormValues) {
    setServerError(null);
    try {
      await mutateAsync(data);
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
          <div className="space-y-1">
            <Label htmlFor="name">Name</Label>
            <Input id="name" {...register("name")} />
            {errors.name && (
              <p className="text-xs text-[hsl(var(--destructive))]">{errors.name.message}</p>
            )}
          </div>
          <div className="space-y-1">
            <Label htmlFor="description">Description (optional)</Label>
            <Input id="description" {...register("description")} />
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={isPending} size="sm">
              {isPending ? "Creating…" : "Create"}
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
  const [showCreate, setShowCreate] = useState(false);

  return (
    <div className="p-6 space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Teams</h1>
          <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">
            Manage your expense-sharing groups
          </p>
        </div>
        {!showCreate && (
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-1" /> New team
          </Button>
        )}
      </div>

      {showCreate && <CreateTeamForm onClose={() => setShowCreate(false)} />}

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
              href={`/teams/${team.id}`}
              className="flex items-center justify-between p-4 rounded-lg border hover:bg-[hsl(var(--muted))] transition-colors"
            >
              <div>
                <p className="font-medium text-sm">{team.name}</p>
                {team.description && (
                  <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
                    {team.description}
                  </p>
                )}
              </div>
              <ArrowRight className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
