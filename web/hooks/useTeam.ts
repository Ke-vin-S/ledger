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

// Matches memberResponse from the backend — no email field
export type Member = {
  id: string;
  user_id: string;
  display_name: string;
  identity_type: string; // "registered" | "anonymous" | "google"
  role: string;
  status: string;
  joined_at?: string;
};

export function isAnonymousMember(m: Member): boolean {
  return m.identity_type === "anonymous";
}

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

// Two-step: create anon user, then add to team
export function useAddAnonymousMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { display_name: string }): Promise<Member> => {
      // Step 1: create the anonymous user
      const anonUser = await api.post<{ id: string; display_name: string; identity_type: string }>(
        "/users/anonymous",
        { display_name: data.display_name },
      );
      // Step 2: add to team
      return api.post<Member>(`/teams/${teamId}/members/anonymous`, { user_id: anonUser.id });
    },
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}

export function useGenerateClaimToken() {
  return useMutation({
    mutationFn: (userId: string) =>
      api.post<{ claim_url: string; token: string; expires_at: string }>(
        `/users/anonymous/${userId}/claim-token`,
      ),
  });
}

export function useRemoveMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) =>
      api.delete(`/teams/${teamId}/members/${userId}`),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}

// Backend only accepts { email } — role changes go through PATCH /:uid/role
export function useInviteMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string }) =>
      api.post(`/teams/${teamId}/members/invite`, { email: data.email }),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ["teams", teamId, "members"] }),
  });
}
