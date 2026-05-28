"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Loan, CreateLoanInput } from "@/types/loan.types";
import { API_ENDPOINTS } from "@/constants/api";

export type { Loan, CreateLoanInput };

export function useLoans(direction?: "lent" | "borrowed") {
  return useQuery<Loan[]>({
    queryKey: ["loans", { direction }],
    queryFn: async () => {
      const path = direction
        ? API_ENDPOINTS.loans.listFiltered(direction)
        : API_ENDPOINTS.loans.list;
      const res = await api.get<{ items?: Loan[] } | Loan[]>(path);
      return Array.isArray(res) ? res : (res as { items?: Loan[] }).items ?? [];
    },
  });
}

export function useLoan(id: string) {
  return useQuery<Loan>({
    queryKey: ["loans", id],
    queryFn: () => api.get<Loan>(API_ENDPOINTS.loans.detail(id)),
    enabled: !!id,
  });
}

export function useCreateLoan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateLoanInput) => api.post<Loan>(API_ENDPOINTS.loans.list, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["loans"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useAcknowledgeLoan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (loanId: string) => api.post(API_ENDPOINTS.loans.acknowledge(loanId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["loans"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useDisputeLoan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ loanId, reason }: { loanId: string; reason?: string }) =>
      api.post(API_ENDPOINTS.loans.dispute(loanId), { reason }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["loans"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useLoanClaimText() {
  return useMutation({
    mutationFn: (loanId: string) =>
      api.post<{ text: string }>(API_ENDPOINTS.loans.claimText(loanId)),
  });
}
