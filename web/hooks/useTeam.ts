"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

export type Team = {
  id: string;
  name: string;
  description?: string;
  currency: string;
  is_public: boolean;
  owner_id: string;
  created_at: string;
};

type Member = {
  user_id: string;
  display_name: string;
  email: string;
  role: string;
  status: string;
  joined_at?: string;
};

export function useTeams() {
  return useQuery<Team[]>({
    queryKey: ["teams"],
    queryFn: () => api.get<Team[]>("/teams"),
  });
}

export function useTeam(teamId: string) {
  return useQuery<Team>({
    queryKey: ["teams", teamId],
    queryFn: () => api.get<Team>(`/teams/${teamId}`),
    enabled: !!teamId,
  });
}

export function useTeamMembers(teamId: string) {
  return useQuery<Member[]>({
    queryKey: ["teams", teamId, "members"],
    queryFn: () => api.get<Member[]>(`/teams/${teamId}/members`),
    enabled: !!teamId,
  });
}

export function useCreateTeam() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; description?: string; currency: string; is_public?: boolean }) =>
      api.post<Team>("/teams", data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["teams"] }),
  });
}

export function useInviteMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string; role: string }) =>
      api.post(`/teams/${teamId}/members`, data),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}
