"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

type Notification = {
  id: string;
  type: string;
  entity_type: string;
  entity_id: string;
  payload?: Record<string, unknown>;
  is_read: boolean;
  read_at?: string;
  created_at: string;
};

type NotificationPage = {
  items: Notification[];
  next_cursor?: string;
  has_more: boolean;
};

export function useNotifications(unreadOnly = false) {
  return useQuery<NotificationPage>({
    queryKey: ["notifications", { unreadOnly }],
    queryFn: () =>
      api.get<NotificationPage>(
        `/notifications${unreadOnly ? "?unread=true" : ""}`,
      ),
    refetchInterval: 30_000,
  });
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/notifications/${id}/read`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useMarkAllRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post("/notifications/read-all"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useDismissNotification() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/notifications/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
