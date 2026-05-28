"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

type Split = {
  id: string;
  user_id: string;
  share_amount: number;
  share_units?: number;
};

export type Expense = {
  id: string;
  scope: string;
  team_id?: string;
  title: string;
  amount: number;
  currency: string;
  split_method?: string;
  expense_date: string;
  receipt_url?: string;
  note?: string;
  version: number;
  is_void: boolean;
  void_reason?: string;
  paid_by: string;
  created_by: string;
  created_at: string;
  splits?: Split[];
};

export type CreateExpenseInput = {
  title: string;
  amount: number;
  currency: string;
  split_method: string;
  expense_date: string;
  paid_by?: string;
  note?: string;
  splits?: { user_id: string; share_amount?: number; share_units?: number }[];
};

export function useExpenses(teamId: string) {
  return useQuery<Expense[]>({
    queryKey: ["teams", teamId, "expenses"],
    queryFn: async () => {
      const res = await api.get<{ items?: Expense[] } | Expense[]>(`/teams/${teamId}/expenses`);
      return Array.isArray(res) ? res : (res as { items?: Expense[] }).items ?? [];
    },
    enabled: !!teamId,
  });
}

export function useExpense(expenseId: string) {
  return useQuery<Expense>({
    queryKey: ["expenses", expenseId],
    queryFn: () => api.get<Expense>(`/expenses/${expenseId}`),
    enabled: !!expenseId,
  });
}

export function useCreateExpense(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateExpenseInput) =>
      api.post<Expense>(`/teams/${teamId}/expenses`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["teams", teamId, "expenses"] });
      qc.invalidateQueries({ queryKey: ["teams", teamId, "balances"] });
      qc.invalidateQueries({ queryKey: ["users", "me"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useCorrectExpense(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ expenseId, data }: { expenseId: string; data: Partial<CreateExpenseInput> & { correction_reason?: string } }) =>
      api.patch<Expense>(`/teams/${teamId}/expenses/${expenseId}`, data),
    onSuccess: (_, { expenseId }) => {
      qc.invalidateQueries({ queryKey: ["teams", teamId, "expenses"] });
      qc.invalidateQueries({ queryKey: ["expenses", expenseId] });
      qc.invalidateQueries({ queryKey: ["expenses", expenseId, "history"] });
      qc.invalidateQueries({ queryKey: ["teams", teamId, "balances"] });
    },
  });
}

export function useVoidExpense(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ expenseId, reason }: { expenseId: string; reason?: string }) =>
      api.delete(`/teams/${teamId}/expenses/${expenseId}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["teams", teamId, "expenses"] });
      qc.invalidateQueries({ queryKey: ["teams", teamId, "balances"] });
    },
  });
}
