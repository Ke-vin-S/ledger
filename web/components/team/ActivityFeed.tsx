"use client";

import { useTeamActivityFeed } from "@/hooks/useGraphQL";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { QueryBoundary } from "@/components/query-boundary";

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
  const query = useTeamActivityFeed(teamId, { limit: 20 });

  return (
    <QueryBoundary
      {...query}
      isEmpty={(data) => data.teamActivityFeed.items.length === 0}
      emptyMessage="Activity will appear here as your team gets moving."
      className="min-h-40"
    >
      {(data) => (
        <ol className="space-y-4" aria-live="polite">
          {data.teamActivityFeed.items.map((item) => (
            <li key={item.id} className="flex items-start gap-3 text-sm">
              <div
                className="mt-2 h-2 w-2 shrink-0 rounded-full bg-primary"
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <p className="leading-snug">
                  {item.actorId ? (
                    <span className="font-medium">{item.actorId}</span>
                  ) : (
                    <span className="text-muted-foreground">System</span>
                  )}{" "}
                  {actionLabel[item.action] ?? item.action}
                </p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  <DateDisplay iso={item.createdAt} withTime />
                </p>
              </div>
            </li>
          ))}
        </ol>
      )}
    </QueryBoundary>
  );
}
