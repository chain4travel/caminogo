// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************

// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"

	"github.com/chain4travel/caminogo/database/memdb"
	"github.com/chain4travel/caminogo/database/prefixdb"
	"github.com/chain4travel/caminogo/database/versiondb"
	"github.com/chain4travel/caminogo/ids"
	"github.com/stretchr/testify/assert"
)

func TestResetHeightIndex(t *testing.T) {
	db := memdb.New()
	vdb := versiondb.New(db)
	heightDB := prefixdb.New(heightIndexPrefix, db)
	heightDB.Put([]byte("key"), []byte("value"))
	hi := NewHeightIndex(heightDB, vdb)
	testId := ids.GenerateTestID()
	hi.SetBlockIDAtHeight(0, testId)
	hi.SetCheckpoint(testId)

	err := hi.ResetHeightIndex()
	assert.NoError(t, err, "No error expected to be thrown by ResetHeightIndex")

	// ensure heightDB has been resetted
	x, _ := hi.GetBlockIDAtHeight(0)
	assert.Equal(t, ids.Empty, x)

	// ensure metadataDB is also resetted
	_, err2 := hi.GetCheckpoint()
	assert.Error(t, err2)

}
