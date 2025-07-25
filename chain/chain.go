package chain

import "context"

type MempoolXray interface {
	// Update your internal state of the mempool
	Update(ctx context.Context)
	Displays() []string
	// Name returns the display name for this chain (e.g., "Ethereum - https://rpc.endpoint")
	Name() string
}
