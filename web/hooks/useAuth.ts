"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { setAccessToken } from "@/lib/auth";
import { useRouter } from "next/navigation";
import type { User } from "@/types/user.types";
import { API_ENDPOINTS } from "@/constants/api";
import { ROUTES } from "@/constants/routes";

export function useMe() {
  return useQuery<User>({
    queryKey: ["users", "me"],
    queryFn: () => api.get<User>(API_ENDPOINTS.users.me),
    retry: false,
  });
}

export function useLogout() {
  const qc = useQueryClient();
  const router = useRouter();
  return useMutation({
    mutationFn: () => api.post(API_ENDPOINTS.auth.logout),
    onSettled: () => {
      setAccessToken(null);
      qc.clear();
      router.push(ROUTES.login);
    },
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { display_name: string }) =>
      api.patch<User>(API_ENDPOINTS.users.me, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users", "me"] }),
  });
}``

export function useUpdateCurrencyPref() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (currency_pref: string) =>
      api.patch<User>(API_ENDPOINTS.users.me, { currency_pref }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users", "me"] }),
  });
}
