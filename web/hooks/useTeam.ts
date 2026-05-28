"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Team, Member } from "@/types/team.types";
import { API_ENDPOINTS } from "@/constants/api";

export type { Team, Member };

export function isAnonymousMember(m: Member): boolean {
  return m.identity_type === "anonymous";
}

export function useTeams() {
  return useQuery<Team[]>({
    queryKey: ["teams"],
    queryFn: () => api.get<Team[]>(API_ENDPOINTS.teams.list),
  });
}

export function useTeam(teamId: string) {
  return useQuery<Team>({
    queryKey: ["teams", teamId],
    queryFn: () => api.get<Team>(API_ENDPOINTS.teams.detail(teamId)),
    enabled: !!teamId,
  });
}

export function useTeamMembers(teamId: string) {
  return useQuery<Member[]>({
    queryKey: ["teams", teamId, "members"],
    queryFn: () => api.get<Member[]>(API_ENDPOINTS.teams.members(teamId)),
    enabled: !!teamId,
  });
}

export function useCreateTeam() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      description?: string;
      currency: string;
      is_public?: boolean;
    }) => api.post<Team>(API_ENDPOINTS.teams.list, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["teams"] }),
  });
}

export function useAddAnonymousMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { display_name: string }): Promise<Member> => {
      const anonUser = await api.post<{ id: string; display_name: string; identity_type: string }>(
        API_ENDPOINTS.users.anonymous,
        { display_name: data.display_name },
      );
      return api.post<Member>(API_ENDPOINTS.teams.addAnonymous(teamId), { user_id: anonUser.id });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}

export function useGenerateClaimToken() {
  return useMutation({
    mutationFn: (userId: string) =>
      api.post<{ claim_url: string; token: string; expires_at: string }>(
        API_ENDPOINTS.users.anonClaimToken(userId),
      ),
  });
}

export function useRemoveMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) =>
      api.delete(API_ENDPOINTS.teams.removeMember(teamId, userId)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}

export function useInviteMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string }) =>
      api.post(API_ENDPOINTS.teams.invite(teamId), { email: data.email }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}
