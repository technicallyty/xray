package eth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCall(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	endpoint := "https://ethereum-rpc.publicnode.com"

	client, err := NewEthereumRPCClient(endpoint)
	require.NoError(t, err)

	res, err := client.TxPoolContent(ctx)
	require.NoError(t, err)
	fmt.Println(res)
}
