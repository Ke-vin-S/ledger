"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { setAccessToken } from "@/lib/auth";
import { useRouter } from "next/navigation";

type User = {
  id: string;
  email: string;
  display_name: string;
  currency_pref: string;
  timezone: string;
  avatar_url?: string;
  identity_type: string;
};

export function useMe() {
  return useQuery<User>({
    queryKey: ["users", "me"],
    queryFn: () => api.get<User>("/users/me"),
    retry: false,
  });
}

export function useLogout() {
  const qc = useQueryClient();
  const router = useRouter();
  return useMutation({
    mutationFn: () => api.post("/auth/logout"),
    onSettled: () => {
      setAccessToken(null);
      qc.clear();
      router.push("/login");
    },
  });
}
