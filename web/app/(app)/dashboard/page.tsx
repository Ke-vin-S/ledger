"use client";

import { useDashboardAggregates } from "@/hooks/useGraphQL";
import { useTeams } from "@/hooks/useTeam";
import { CurrencyAmount } from "@/components/shared/CurrencyAmount";
import { Skeleton } from "@/components/shared/Skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import Link from "next/link";
import { ArrowRight } from "lucide-react";

function BalanceSummary() {
  const { data, isLoading } = useDashboardAggregates();
  const agg = data?.dashboardAggregates;

  if (isLoading) {
    return (
      <div className="grid grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-24" />
        ))}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-[hsl(var(--muted-foreground))]">
            You are owed
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CurrencyAmount
            amount={agg?.totalOwed ?? 0}
            signed
            className="text-2xl font-bold"
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-[hsl(var(--muted-foreground))]">
            You owe
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CurrencyAmount
            amount={-(agg?.totalOwing ?? 0)}
            signed
            className="text-2xl font-bold"
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-[hsl(var(--muted-foreground))]">
            Net balance
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CurrencyAmount
            amount={agg?.netBalance ?? 0}
            signed
            className="text-2xl font-bold"
          />
        </CardContent>
      </Card>
    </div>
  );
}

function TeamsList() {
  const { data: teams, isLoading } = useTeams();

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-14" />
        ))}
      </div>
    );
  }

  if (!teams?.length) {
    return (
      <p className="text-sm text-[hsl(var(--muted-foreground))] py-4">
        No teams yet.{" "}
        <Link href="/teams" className="underline">
          Create one
        </Link>
        .
      </p>
    );
  }

  return (
    <div className="space-y-2">
      {teams.map((team) => (
        <Link
          key={team.id}
          href={`/teams/${team.id}`}
          className="flex items-center justify-between p-3 rounded-lg border hover:bg-[hsl(var(--muted))] transition-colors"
        >
          <span className="font-medium text-sm">{team.name}</span>
          <ArrowRight className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
        </Link>
      ))}
    </div>
  );
}

export default function DashboardPage() {
  return (
    <div className="p-8 space-y-8 max-w-4xl">
      <div>
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">
          Your financial overview
        </p>
      </div>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Balance summary</h2>
        <BalanceSummary />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Your teams</h2>
        <TeamsList />
      </section>
    </div>
  );
}
