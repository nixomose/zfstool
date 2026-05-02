package api

import "context"

// Client is the read-only API surface used by GUI and web proxy layers.
type Client interface {
	GetHost(ctx context.Context) (*HostInfo, error)
	ListPools(ctx context.Context) ([]PoolSummary, error)
	PoolStatus(ctx context.Context, pool string) (*PoolStatus, error)
	PoolHistory(ctx context.Context, pool string, offset, limit int) ([]PoolHistoryEntry, error)
	ListDatasets(ctx context.Context, pool string) ([]DatasetRow, error)
}
