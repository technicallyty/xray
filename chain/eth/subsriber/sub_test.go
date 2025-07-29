package subscriber

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestThing(t *testing.T) {
	NodeEndpoint := "http://localhost:8545"
	ctx := context.Background()

	baseClient, err := rpc.Dial(NodeEndpoint)
	if err != nil {
		log.Fatalln(err)
	}

	ethClient, err := ethclient.Dial(NodeEndpoint)
	if err != nil {
		log.Fatalln(err)
	}

	chainID, err := ethClient.NetworkID(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Chain ID: ", chainID)

	subscriber := gethclient.New(baseClient)

	txChannel := make(chan *types.Transaction)
	_, err = subscriber.SubscribeFullPendingTransactions(ctx, txChannel)

	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		baseClient.Close()
		ethClient.Close()
	}()

	for tx := range txChannel {
		fmt.Printf("Hash: %s  Nonce: %d/n", tx.Hash().Hex(), tx.Nonce())
	}
}
