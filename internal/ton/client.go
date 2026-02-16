package ton

// TON Lite Server client placeholder.
// In production, use a TON SDK (tonutils-go or similar) to:
// 1. Connect to lite server
// 2. Scan blocks for transactions to hot wallet address
// 3. Parse comments/payloads from internal messages
// 4. Return parsed Transaction structs

import (
	"context"
	"time"
)

type LiteClient struct {
	host string
	port int
	key  string
}

type TxInfo struct {
	Hash        string
	FromAddr    string
	ToAddr      string
	AmountNano  int64
	Comment     string
	BlockTime   time.Time
	Lt          uint64
}

func NewLiteClient(host string, port int, key string) *LiteClient {
	return &LiteClient{host: host, port: port, key: key}
}

// GetTransactions returns new incoming transactions to the given address
// since the last known logical time (lt).
// TODO: Implement actual lite server communication.
func (c *LiteClient) GetTransactions(ctx context.Context, address string, lastLt uint64) ([]TxInfo, error) {
	// Placeholder — in production:
	// 1. tonutils-go / liteclient connect
	// 2. GetAccountState to get latest lt
	// 3. GetTransactions from lastLt to latest
	// 4. Filter incoming, parse comments
	return nil, nil
}

// SendTON sends TON from hot wallet to destination.
// Used for payouts and refunds.
// TODO: Implement actual sending via lite client or toncenter API.
func (c *LiteClient) SendTON(ctx context.Context, fromSecret, toAddress string, amountNano int64, comment string) (string, error) {
	// Placeholder
	return "", nil
}
