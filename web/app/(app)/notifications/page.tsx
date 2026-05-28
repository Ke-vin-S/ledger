"use client";

import {
  useNotifications,
  useMarkAllRead,
  useMarkNotificationRead,
  useDismissNotification,
} from "@/hooks/useNotifications";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { Skeleton } from "@/components/shared/Skeleton";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { X } from "lucide-react";

export default function NotificationsPage() {
  const { data, isLoading } = useNotifications();
  const { mutate: markAllRead, isPending: markingAll } = useMarkAllRead();
  const { mutate: markRead } = useMarkNotificationRead();
  const { mutate: dismiss } = useDismissNotification();

  const items = data?.items ?? [];
  const unread = items.filter((n) => !n.is_read);

  return (
    <div className="p-8 space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <p className="text-sm text-[hsl(var(--muted-foreground))]">{unread.length} unread</p>
        {unread.length > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => markAllRead()}
            disabled={markingAll}
          >
            Mark all read
          </Button>
        )}
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : !items.length ? (
        <p className="text-sm text-[hsl(var(--muted-foreground))]">
          You&apos;re all caught up!
        </p>
      ) : (
        <ul className="space-y-2">
          {items.map((n) => (
            <li
              key={n.id}
              className={cn(
                "flex items-start gap-3 p-3 rounded-lg border",
                !n.is_read && "bg-[hsl(var(--muted))]",
              )}
            >
              {!n.is_read && (
                <div className="h-2 w-2 rounded-full bg-[hsl(var(--primary))] mt-1.5 flex-shrink-0" />
              )}
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium">{n.type.replace(/_/g, " ")}</p>
                <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
                  <DateDisplay iso={n.created_at} withTime />
                </p>
              </div>
              <div className="flex gap-1 flex-shrink-0">
                {!n.is_read && (
                  <button
                    onClick={() => markRead(n.id)}
                    className="text-xs text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] px-2 py-1 rounded"
                  >
                    Mark read
                  </button>
                )}
                <button
                  onClick={() => dismiss(n.id)}
                  className="p-1 rounded hover:bg-[hsl(var(--muted))] transition-colors"
                  aria-label="Dismiss"
                >
                  <X className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
