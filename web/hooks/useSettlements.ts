"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Settlement, Balance } from "@/types/settlement.types";
import { API_ENDPOINTS } from "@/constants/api";
import { invalidateExpenseGraph } from "@/lib/queryKeys";

export function useExpenseSettlements(expenseId: string) {
  return useQuery<Settlement[]>({
    queryKey: ["expenses", expenseId, "settlements"],
    queryFn: () => api.get<Settlement[]>(API_ENDPOINTS.expenses.settlements(expenseId)),
    enabled: !!expenseId,
  });
}

export function useTeamBalances(teamId: string) {
  return useQuery<Balance[]>({
    queryKey: ["teams", teamId, "balances"],
    queryFn: () => api.get<Balance[]>(API_ENDPOINTS.teams.balances(teamId)),
    enabled: !!teamId,
  });
}

export function useMyBalances() {
  return useQuery<Balance[]>({
    queryKey: ["balances"],
    queryFn: () => api.get<Balance[]>(API_ENDPOINTS.balances),
  });
}

export function useRecordSettlement(teamId: string, expenseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      payer_id: string;
      payee_id: string;
      amount: number;
      method: string;
      method_note?: string;
      settled_on: string;
    }) => api.post<Settlement>(API_ENDPOINTS.expenses.settlements(expenseId), data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["expenses", expenseId, "settlements"] });
      qc.invalidateQueries({ queryKey: ["teams", teamId, "balances"] });
      qc.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

export function useConfirmSettlement(teamId: string, expenseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settlementId: string) =>
      api.post(API_ENDPOINTS.settlements.confirm(settlementId)),
    onSuccess: () => {
      invalidateExpenseGraph(qc, { teamId, expenseId });
    },
  });
}

export function useDisputeSettlement(teamId: string, expenseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ settlementId, reason }: { settlementId: string; reason?: string }) =>
      api.post(API_ENDPOINTS.settlements.dispute(settlementId), { reason }),
    onSuccess: () => {
      invalidateExpenseGraph(qc, { teamId, expenseId });
    },
  });
}
