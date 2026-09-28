import type { QueryClient } from "@tanstack/react-query";

export function invalidateExpenseGraph(
  queryClient: QueryClient,
  { teamId, expenseId }: { teamId?: string; expenseId?: string },
): void {
  void queryClient.invalidateQueries({ queryKey: ["expenses"] });
  if (teamId) {
    void queryClient.invalidateQueries({ queryKey: ["teams", teamId, "expenses"] });
    void queryClient.invalidateQueries({ queryKey: ["teams", teamId, "balances"] });
  }
  if (expenseId) {
    void queryClient.invalidateQueries({ queryKey: ["expenses", expenseId] });
  }
  void queryClient.invalidateQueries({ queryKey: ["balances"] });
  void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
}
