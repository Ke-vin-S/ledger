"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

type Settlement = {
  id: string;
  expense_id: string;
  payer_id: string;
  payee_id: string;
  amount: number;
  status: string;
  recorded_at: string;
  confirmed_at?: string;
};

type Balance = {
  counterparty_id: string;
  counterparty_name: string;
  net_amount: number;
};

export function useExpenseSettlements(expenseId: string) {
  return useQuery<Settlement[]>({
    queryKey: ["expenses", expenseId, "settlements"],
    queryFn: () => api.get<Settlement[]>(`/expenses/${expenseId}/settlements`),
    enabled: !!expenseId,
  });
}

export function useTeamBalances(teamId: string) {
  return useQuery<Balance[]>({
    queryKey: ["teams", teamId, "balances"],
    queryFn: () => api.get<Balance[]>(`/teams/${teamId}/balances`),
    enabled: !!teamId,
  });
}

export function useMyBalances() {
  return useQuery<Balance[]>({
    queryKey: ["balances"],
    queryFn: () => api.get<Balance[]>("/balances"),
  });
}

export function useRecordSettlement(expenseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { payee_id: string; amount: number }) =>
      api.post<Settlement>(`/expenses/${expenseId}/settlements`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["expenses", expenseId, "settlements"] });
      qc.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

export function useConfirmSettlement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settlementId: string) =>
      api.post(`/settlements/${settlementId}/confirm`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}
