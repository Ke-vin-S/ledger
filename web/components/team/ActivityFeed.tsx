"use client";

import { useTeamActivityFeed } from "@/hooks/useGraphQL";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Skeleton } from "@/components/shared/Skeleton";

type Props = { teamId: string };

const actionLabel: Record<string, string> = {
  "expense.created": "created an expense",
  "expense.corrected": "corrected an expense",
  "expense.voided": "voided an expense",
  "member.invited": "invited a member",
  "member.joined": "joined the team",
  "member.removed": "was removed",
  "settlement.recorded": "recorded a settlement",
  "settlement.confirmed": "confirmed a settlement",
  "flag.opened": "raised a flag",
  "flag.resolved": "resolved a flag",
};

export function ActivityFeed({ teamId }: Props) {
  const { data, isLoading } = useTeamActivityFeed(teamId, { limit: 20 });
  const items = data?.teamActivityFeed.items ?? [];

  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-10" />
        ))}
      </div>
    );
  }

  if (!items.length) {
    return (
      <p className="text-sm text-[hsl(var(--muted-foreground))] py-4">
        No activity yet.
      </p>
    );
  }

  return (
    <ol className="space-y-3">
      {items.map((item) => (
        <li key={item.id} className="flex items-start gap-3 text-sm">
          <div className="flex-shrink-0 h-2 w-2 rounded-full bg-[hsl(var(--muted-foreground))] mt-2" />
          <div className="flex-1 min-w-0">
            <p className="leading-snug">
              {item.actorId ? (
                <span className="font-medium">{item.actorId}</span>
              ) : (
                <span className="text-[hsl(var(--muted-foreground))]">System</span>
              )}{" "}
              {actionLabel[item.action] ?? item.action}
            </p>
            <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
              <DateDisplay iso={item.createdAt} withTime />
            </p>
          </div>
        </li>
      ))}
    </ol>
  );
}
