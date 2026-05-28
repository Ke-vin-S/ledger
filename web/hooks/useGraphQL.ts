"use client";

import { useQuery } from "@tanstack/react-query";
import { getGraphQLClient } from "@/lib/graphql/client";
import {
  TEAM_ACTIVITY_FEED,
  DASHBOARD_AGGREGATES,
  EXPENSE_HISTORY,
} from "@/lib/graphql/queries";
import type {
  TeamActivityFeedQuery,
  TeamActivityFeedQueryVariables,
  DashboardAggregatesQuery,
  ExpenseHistoryQuery,
  ExpenseHistoryQueryVariables,
} from "@/lib/graphql/types";

export function useTeamActivityFeed(
  teamId: string,
  options?: { limit?: number; cursor?: string },
) {
  return useQuery<TeamActivityFeedQuery>({
    queryKey: ["teams", teamId, "activity", options],
    queryFn: () => {
      const vars: TeamActivityFeedQueryVariables = {
        teamId,
        limit: options?.limit,
        cursor: options?.cursor,
      };
      return getGraphQLClient().request<TeamActivityFeedQuery>(
        TEAM_ACTIVITY_FEED,
        vars,
      );
    },
    enabled: !!teamId,
  });
}

export function useDashboardAggregates() {
  return useQuery<DashboardAggregatesQuery>({
    queryKey: ["dashboard"],
    queryFn: () =>
      getGraphQLClient().request<DashboardAggregatesQuery>(DASHBOARD_AGGREGATES),
  });
}

export function useExpenseHistory(expenseId: string) {
  return useQuery<ExpenseHistoryQuery>({
    queryKey: ["expenses", expenseId, "history"],
    queryFn: () => {
      const vars: ExpenseHistoryQueryVariables = { expenseId };
      return getGraphQLClient().request<ExpenseHistoryQuery>(
        EXPENSE_HISTORY,
        vars,
      );
    },
    enabled: !!expenseId,
  });
}
