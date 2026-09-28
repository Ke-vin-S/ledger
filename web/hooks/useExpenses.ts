"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Expense, CreateExpenseInput } from "@/types/expense.types";
import { API_ENDPOINTS } from "@/constants/api";
import { invalidateExpenseGraph } from "@/lib/queryKeys";

export type { Expense, CreateExpenseInput };

export function useMyExpenses() {
  return useQuery<Expense[]>({
    queryKey: ["expenses", "mine"],
    queryFn: () => api.get<Expense[]>(API_ENDPOINTS.expenses.list),
  });
}

export function useExpenses(teamId: string) {
  return useQuery<Expense[]>({
    queryKey: ["teams", teamId, "expenses"],
    queryFn: async () => {
      const res = await api.get<{ items?: Expense[] } | Expense[]>(
        API_ENDPOINTS.teams.expenses(teamId),
      );
      return Array.isArray(res) ? res : (res as { items?: Expense[] }).items ?? [];
    },
    enabled: !!teamId,
  });
}

export function useExpense(expenseId: string) {
  return useQuery<Expense>({
    queryKey: ["expenses", expenseId],
    queryFn: () => api.get<Expense>(API_ENDPOINTS.expenses.detail(expenseId)),
    enabled: !!expenseId,
  });
}

export function useCreateExpense(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateExpenseInput) =>
      api.post<Expense>(API_ENDPOINTS.teams.expenses(teamId), data),
    onSuccess: () => {
      invalidateExpenseGraph(qc, { teamId });
      void qc.invalidateQueries({ queryKey: ["users", "me"] });
    },
  });
}

export function useCorrectExpense(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      expenseId,
      data,
    }: {
      expenseId: string;
      data: Partial<CreateExpenseInput> & { correction_reason?: string };
    }) => api.patch<Expense>(API_ENDPOINTS.teams.expense(teamId, expenseId), data),
    onSuccess: (_, { expenseId }) => {
      invalidateExpenseGraph(qc, { teamId, expenseId });
    },
  });
}

export function useVoidExpense(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ expenseId, reason }: { expenseId: string; reason: string }) =>
      api.patch(API_ENDPOINTS.expenses.void(expenseId), { reason }),
    onSuccess: (_, { expenseId }) => {
      invalidateExpenseGraph(qc, { teamId, expenseId });
    },
  });
}
