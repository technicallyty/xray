package chain

import "context"

type MempoolXray interface {
	// Start starts the service. Services should use the start method to kick off polling/subscribing in a go routine.
	// Services should update their state in Start.
	Start(ctx context.Context)
	// Displays returns data to display. This should display the data collected in Start.
	Displays() []string
	// Name is the name of the service.
	Name() string
}
