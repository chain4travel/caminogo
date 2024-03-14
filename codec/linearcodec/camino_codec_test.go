// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package linearcodec

import (
	"testing"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
)

func TestVectorsCamino(t *testing.T) {
	for _, test := range codec.Tests {
		c := NewCaminoDefault(mockable.MaxTime)
		test(c, t)
	}
}

func TestMultipleTagsCamino(t *testing.T) {
	for _, test := range codec.MultipleTagsTests {
		c := NewCamino(mockable.MaxTime, []string{"tag1", "tag2"}, DefaultMaxSliceLength)
		test(c, t)
	}
}

func TestVersionCamino(t *testing.T) {
	for _, test := range codec.VersionTests {
		c := NewCaminoDefault(mockable.MaxTime)
		test(c, t)
	}
}
