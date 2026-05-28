"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

export type Loan = {
  id: string;
  direction: "lent" | "borrowed";
  amount: number;
  currency: string;
  counterparty_id: string;
  counterparty_name: string;
  note?: string;
  status: "outstanding" | "partially_repaid" | "settled" | "disputed";
  loan_date: string;
  created_at: string;
  repayments?: Repayment[];
};

export type Repayment = {
  id: string;
  amount: number;
  repaid_at: string;
  note?: string;
};

export type CreateLoanInput = {
  direction: "lent" | "borrowed";
  amount: number;
  currency: string;
  counterparty_id?: string;
  counterparty_name?: string;
  note?: string;
  loan_date: string;
};

export function useLoans(direction?: "lent" | "borrowed") {
  return useQuery<Loan[]>({
    queryKey: ["loans", { direction }],
    queryFn: async () => {
      const path = direction ? `/loans?direction=${direction}` : "/loans";
      const res = await api.get<{ items?: Loan[] } | Loan[]>(path);
      return Array.isArray(res) ? res : (res as { items?: Loan[] }).items ?? [];
    },
  });
}

export function useLoan(id: string) {
  return useQuery<Loan>({
    queryKey: ["loans", id],
    queryFn: () => api.get<Loan>(`/loans/${id}`),
    enabled: !!id,
  });
}

export function useCreateLoan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateLoanInput) => api.post<Loan>("/loans", data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["loans"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useAcknowledgeLoan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (loanId: string) => api.post(`/loans/${loanId}/acknowledge`),
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
      api.post(`/loans/${loanId}/dispute`, { reason }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["loans"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useLoanClaimText() {
  return useMutation({
    mutationFn: (loanId: string) => api.post<{ text: string }>(`/loans/${loanId}/claim-text`),
  });
}
