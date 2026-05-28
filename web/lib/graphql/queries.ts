export const TEAM_ACTIVITY_FEED = `
  query TeamActivityFeed($teamId: ID!, $limit: Int, $cursor: String) {
    teamActivityFeed(teamId: $teamId, limit: $limit, cursor: $cursor) {
      items {
        id
        action
        actorId
        entityType
        entityId
        meta
        createdAt
      }
      nextCursor
      hasMore
    }
  }
`;

export const DASHBOARD_AGGREGATES = `
  query DashboardAggregates {
    dashboardAggregates {
      totalOwed
      totalOwing
      netBalance
    }
  }
`;

export const EXPENSE_HISTORY = `
  query ExpenseHistory($expenseId: ID!) {
    expenseHistory(expenseId: $expenseId) {
      id
      expenseId
      version
      snapshot
      correctedBy
      correctionReason
      createdAt
    }
  }
`;
