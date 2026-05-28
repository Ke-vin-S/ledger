package graph

// Resolver holds the stores needed by each query resolver.
// It is the dependency injection root for the GraphQL layer.
type Resolver struct {
	activityStore ActivityFeedStore
	dashStore     DashboardStore
	historyStore  ExpenseHistoryStore
}

func NewResolver(act ActivityFeedStore, dash DashboardStore, hist ExpenseHistoryStore) *Resolver {
	return &Resolver{
		activityStore: act,
		dashStore:     dash,
		historyStore:  hist,
	}
}
