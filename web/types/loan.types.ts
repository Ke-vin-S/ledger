export type Repayment = {
  id: string;
  amount: number;
  repaid_at: string;
  note?: string;
};

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

export type CreateLoanInput = {
  direction: "lent" | "borrowed";
  amount: number;
  currency: string;
  counterparty_id?: string;
  counterparty_name?: string;
  note?: string;
  loan_date: string;
};
