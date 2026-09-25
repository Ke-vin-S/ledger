"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import {
  Bell,
  CheckCheck,
  CircleDollarSign,
  Flag,
  Info,
  UsersRound,
  X,
  type LucideIcon,
} from "lucide-react";
import {
  useDismissNotification,
  useMarkAllRead,
  useMarkNotificationRead,
  useNotifications,
} from "@/hooks/useNotifications";
import { DateDisplay } from "@/components/shared/DateDisplay";
import { PageHeader } from "@/components/shared/PageHeader";
import { QueryBoundary } from "@/components/query-boundary";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import { useToast } from "@/components/ui/toast";
import { ApiRequestError } from "@/lib/api";
import { ROUTES } from "@/constants/routes";
import type { Notification } from "@/types/notification.types";

const NOTIFICATION_LABELS: Record<string, string> = {
  expense_created: "An expense was added",
  expense_corrected: "An expense was corrected",
  expense_voided: "An expense was voided",
  member_invited: "A member was invited",
  member_joined: "A member joined the team",
  settlement_recorded: "A settlement was recorded",
  settlement_confirmed: "A settlement was confirmed",
  flag_opened: "A flag was raised",
  flag_resolved: "A flag was resolved",
};
const NOTIFICATION_ICONS: Record<string, LucideIcon> = {
  expense_created: CircleDollarSign,
  expense_corrected: CircleDollarSign,
  expense_voided: CircleDollarSign,
  member_invited: UsersRound,
  member_joined: UsersRound,
  settlement_recorded: CheckCheck,
  settlement_confirmed: CheckCheck,
  flag_opened: Flag,
  flag_resolved: Flag,
};

function notificationLabel(notification: Notification): string {
  return (
    NOTIFICATION_LABELS[notification.type] ??
    notification.type
      .replace(/[._]/g, " ")
      .replace(/\b\w/g, (letter) => letter.toUpperCase())
  );
}

function notificationHref(notification: Notification): string | undefined {
  if (notification.entity_type === "loan" && notification.entity_id)
    return ROUTES.loanDetail(notification.entity_id);
  if (notification.entity_type === "team" && notification.entity_id)
    return ROUTES.team(notification.entity_id);
  return undefined;
}

function dayLabel(iso: string): string {
  const date = new Date(iso);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (date.toDateString() === today.toDateString()) return "Today";
  if (date.toDateString() === yesterday.toDateString()) return "Yesterday";
  return date.toLocaleDateString("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
  });
}

export default function NotificationsPage() {
  const query = useNotifications();
  const markAllRead = useMarkAllRead();
  const markRead = useMarkNotificationRead();
  const dismiss = useDismissNotification();
  const { toast } = useToast();
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const totalPages = Math.max(
    1,
    Math.ceil((query.data?.items.length ?? 0) / pageSize),
  );
  const visibleItems = (query.data?.items ?? []).slice(
    (page - 1) * pageSize,
    page * pageSize,
  );
  const groups = useMemo(() => {
    const grouped = new Map<string, Notification[]>();
    for (const notification of visibleItems) {
      const label = dayLabel(notification.created_at);
      const current = grouped.get(label) ?? [];
      current.push(notification);
      grouped.set(label, current);
    }
    return Array.from(grouped.entries());
  }, [visibleItems]);

  function handleMarkAllRead() {
    markAllRead.mutate(undefined, {
      onSuccess: () => toast({ title: "All caught up", variant: "success" }),
      onError: (error) =>
        toast({
          title: "Could not mark notifications read",
          description:
            error instanceof ApiRequestError
              ? error.error.message
              : "Try again.",
          variant: "destructive",
        }),
    });
  }

  function handleMarkRead(id: string) {
    markRead.mutate(id, {
      onError: (error) =>
        toast({
          title: "Could not mark notification read",
          description:
            error instanceof ApiRequestError
              ? error.error.message
              : "Try again.",
          variant: "destructive",
        }),
    });
  }

  function handleDismiss(id: string) {
    dismiss.mutate(id, {
      onError: (error) =>
        toast({
          title: "Could not dismiss notification",
          description:
            error instanceof ApiRequestError
              ? error.error.message
              : "Try again.",
          variant: "destructive",
        }),
    });
  }

  return (
    <main className="mx-auto max-w-4xl space-y-8 p-4 md:p-8">
      <PageHeader
        eyebrow="Stay in the loop"
        title="Notifications"
        description="Important changes from your teams, expenses, and settlements."
        action={
          query.data && query.data.items.some((item) => !item.is_read) ? (
            <Button
              type="button"
              variant="outline"
              onClick={handleMarkAllRead}
              disabled={markAllRead.isPending}
            >
              <CheckCheck className="h-4 w-4" aria-hidden="true" /> Mark all
              read
            </Button>
          ) : undefined
        }
      />
      <QueryBoundary
        {...query}
        isEmpty={(data) => data.items.length === 0}
        emptyMessage="You are all caught up. New activity will appear here."
        className="min-h-64"
      >
        {(data) => {
          const unread = data.items.filter((item) => !item.is_read).length;
          return (
            <div className="space-y-6">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Bell className="h-4 w-4 text-primary" aria-hidden="true" />
                {unread
                  ? `${unread} unread ${unread === 1 ? "notification" : "notifications"}`
                  : "All notifications read"}
              </div>
              <div className="space-y-6">
                {groups.map(([label, notifications]) => (
                  <section
                    key={label}
                    aria-labelledby={`notifications-${label}`}
                    className="space-y-2"
                  >
                    <h2
                      id={`notifications-${label}`}
                      className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground"
                    >
                      {label}
                    </h2>
                    <div className="overflow-hidden rounded-xl border bg-card">
                      {notifications.map((notification) => {
                        const Icon =
                          NOTIFICATION_ICONS[notification.type] ?? Info;
                        const href = notificationHref(notification);
                        const content = (
                          <div className="flex items-start gap-3 p-4 transition-colors hover:bg-muted/30">
                            <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                              <Icon className="h-4 w-4" aria-hidden="true" />
                            </span>
                            <div className="min-w-0 flex-1">
                              <p className="text-sm font-semibold">
                                {notificationLabel(notification)}
                              </p>
                              <p className="mt-1 text-xs text-muted-foreground">
                                <DateDisplay
                                  iso={notification.created_at}
                                  withTime
                                />
                              </p>
                            </div>
                            {!notification.is_read ? (
                              <span
                                className="mt-1 size-2 shrink-0 rounded-full bg-primary"
                                aria-label="Unread"
                              />
                            ) : null}
                          </div>
                        );
                        return (
                          <div
                            key={notification.id}
                            className="border-b border-border last:border-b-0"
                          >
                            {href ? (
                              <Link
                                href={href as never}
                                className="block focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                              >
                                {content}
                              </Link>
                            ) : (
                              content
                            )}
                            <div className="flex justify-end gap-1 px-3 pb-3">
                              {!notification.is_read ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="ghost"
                                  onClick={() =>
                                    handleMarkRead(notification.id)
                                  }
                                >
                                  Mark read
                                </Button>
                              ) : null}
                              <Button
                                type="button"
                                size="icon"
                                variant="ghost"
                                aria-label={`Dismiss ${notificationLabel(notification)}`}
                                onClick={() => handleDismiss(notification.id)}
                              >
                                <X className="h-4 w-4" aria-hidden="true" />
                              </Button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </section>
                ))}
              </div>
              {totalPages > 1 ? (
                <Pagination
                  page={page}
                  totalPages={totalPages}
                  onPageChange={setPage}
                />
              ) : null}
            </div>
          );
        }}
      </QueryBoundary>
    </main>
  );
}
