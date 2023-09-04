// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package locked

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/avax"
)

var (
	errInvalidLockState = errors.New("invalid lockState")
	errNestedLocks      = errors.New("shouldn't nest locks")
	ThisTxID            = ids.ID{'t', 'h', 'i', 's', ' ', 't', 'x', ' ', 'i', 'd'}
)

type State byte

const (
	StateUnlocked State = 0b00
	StateLocked   State = 0b01
)

var stateStrings = map[State]string{
	StateUnlocked: "unlocked",
	StateLocked:   "locked",
}

func (ls State) String() string {
	stateString, ok := stateStrings[ls]
	if !ok {
		return fmt.Sprintf("unknownLockState(%d)", ls)
	}
	return stateString
}

func (ls State) Verify() error {
	if ls < StateUnlocked || StateLocked < ls {
		return errInvalidLockState
	}
	return nil
}

func (ls State) IsLocked() bool {
	return StateLocked&ls == StateLocked
}

/**********************  IDs *********************/

type IDs struct {
	LockTxID ids.ID `serialize:"true" json:"depositTxID"`
}

var IDsEmpty = IDs{ids.Empty}

func (lock IDs) LockState() State {
	lockState := StateUnlocked
	if lock.LockTxID != ids.Empty {
		lockState = StateLocked
	}
	return lockState
}

func (lock IDs) Lock(lockState State) IDs {
	if lockState.IsLocked() {
		lock.LockTxID = ThisTxID
	}
	return lock
}

func (lock IDs) Unlock(lockState State) IDs {
	if lockState.IsLocked() {
		lock.LockTxID = ids.Empty
	}
	return lock
}

func (lock *IDs) FixLockID(txID ids.ID, appliedLockState State) {
	switch appliedLockState {
	case StateLocked:
		if lock.LockTxID == ThisTxID {
			lock.LockTxID = txID
		}
	}
}

func (lock IDs) IsLocked() bool {
	return lock.LockTxID != ids.Empty
}

func (lock IDs) IsLockedWith(lockState State) bool {
	return lock.LockState()&lockState == lockState
}

func (lock IDs) IsNewlyLockedWith(lockState State) bool {
	switch lockState {
	case StateLocked:
		return lock.LockTxID == ThisTxID
	}
	return false
}

func (lock *IDs) Match(lockState State, txIDs set.Set[ids.ID]) bool {
	switch lockState {
	case StateLocked:
		return txIDs.Contains(lock.LockTxID)
	}
	return false
}

/**********************  In / Out *********************/

type Out struct {
	IDs                  `serialize:"true" json:"lockIDs"`
	avax.TransferableOut `serialize:"true" json:"output"`
}

func (out *Out) Addresses() [][]byte {
	if addressable, ok := out.TransferableOut.(avax.Addressable); ok {
		return addressable.Addresses()
	}
	return nil
}

func (out *Out) Verify() error {
	if _, nested := out.TransferableOut.(*Out); nested {
		return errNestedLocks
	}
	return out.TransferableOut.Verify()
}

type In struct {
	IDs                 `serialize:"true" json:"lockIDs"`
	avax.TransferableIn `serialize:"true" json:"input"`
}

func (in *In) Verify() error {
	if _, nested := in.TransferableIn.(*In); nested {
		return errNestedLocks
	}
	return in.TransferableIn.Verify()
}
