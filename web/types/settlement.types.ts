export type SettlementMethod = "cash" | "bank_transfer" | "upi" | "card" | "other";

export type Settlement = {
  id: string;
  expense_id: string;
  payer_id: string;
  payee_id: string;
  amount: number;
  method: string;
  method_note?: string;
  status: string;
  recorded_by: string;
  confirmed_by?: string;
  confirmed_at?: string;
  disputed_by?: string;
  disputed_at?: string;
  dispute_reason?: string;
  settled_on: string;
  created_at: string;
};

export type Balance = {
  counterparty_id: string;
  counterparty_name: string;
  net_amount: number;
};
