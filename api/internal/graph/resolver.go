package graph

// Resolver holds the stores needed by each query resolver.
// It is the dependency injection root for the GraphQL layer.
type Resolver struct {
	activityStore ActivityFeedStore
	dashStore     DashboardStore
	historyStore  ExpenseHistoryStore
	memberships   MembershipChecker
	expenses      ExpenseReader
}

func NewResolver(act ActivityFeedStore, dash DashboardStore, hist ExpenseHistoryStore, memberships MembershipChecker, expenses ExpenseReader) *Resolver {
	return &Resolver{
		activityStore: act,
		dashStore:     dash,
		historyStore:  hist,
		memberships:   memberships,
		expenses:      expenses,
	}
}
