export type Split = {
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

export type SplitMethod = "equal" | "exact" | "percentage" | "shares";

export type SplitEntry = {
  user_id: string;
  /** Minor units — used for exact splits */
  share_amount?: number;
  /** Relative weight — used for shares/percentage */
  share_units?: number;
};
