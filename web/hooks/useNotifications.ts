"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { NotificationPage } from "@/types/notification.types";
import { API_ENDPOINTS } from "@/constants/api";
import { NOTIFICATION_POLL_INTERVAL } from "@/constants/config";

export function useNotifications(unreadOnly = false) {
  return useQuery<NotificationPage>({
    queryKey: ["notifications", { unreadOnly }],
    queryFn: () =>
      api.get<NotificationPage>(
        unreadOnly ? API_ENDPOINTS.notifications.unreadList : API_ENDPOINTS.notifications.list,
      ),
    refetchInterval: NOTIFICATION_POLL_INTERVAL,
  });
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(API_ENDPOINTS.notifications.read(id)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useMarkAllRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(API_ENDPOINTS.notifications.readAll),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useDismissNotification() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(API_ENDPOINTS.notifications.dismiss(id)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
