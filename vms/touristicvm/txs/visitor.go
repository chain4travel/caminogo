// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

// Allow vm to execute custom logic against the underlying transaction types.
type Visitor interface {
	BaseTx(*BaseTx) error
	ImportTx(*ImportTx) error
	LockMessengerFundsTx(*LockMessengerFundsTx) error
	CashoutChequeTx(*CashoutChequeTx) error
}