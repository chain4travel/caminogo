// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package linearcodec

import (
	"fmt"
	"reflect"
	"time"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/reflectcodec"
	"github.com/ava-labs/avalanchego/utils/bimap"
)

const (
	firstCustomTypeID = 8192
)

var (
	_ CaminoCodec          = (*caminoLinearCodec)(nil)
	_ codec.Codec          = (*caminoLinearCodec)(nil)
	_ codec.CaminoRegistry = (*caminoLinearCodec)(nil)
	_ codec.GeneralCodec   = (*caminoLinearCodec)(nil)
)

// Codec marshals and unmarshals
type CaminoCodec interface {
	codec.CaminoRegistry
	codec.Codec
	SkipRegistrations(int)
	SkipCustomRegistrations(int)
}

type caminoLinearCodec struct {
	linearCodec
	nextCustomTypeID uint32
}

func NewCamino(durangoTime time.Time, tagNames []string, maxSliceLen uint32) CaminoCodec {
	hCodec := &caminoLinearCodec{
		linearCodec: linearCodec{
			nextTypeID:      0,
			registeredTypes: bimap.New[uint32, reflect.Type](),
		},
		nextCustomTypeID: firstCustomTypeID,
	}
	hCodec.Codec = reflectcodec.New(hCodec, tagNames, durangoTime, maxSliceLen)
	return hCodec
}

// NewDefault is a convenience constructor; it returns a new codec with reasonable default values
func NewCaminoDefault(durangoTime time.Time) CaminoCodec {
	return NewCamino(durangoTime, []string{reflectcodec.DefaultTagName}, DefaultMaxSliceLength)
}

// NewCustomMaxLength is a convenience constructor; it returns a new codec with custom max length and default tags
func NewCaminoCustomMaxLength(durangoTime time.Time, maxSliceLen uint32) CaminoCodec {
	return NewCamino(durangoTime, []string{reflectcodec.DefaultTagName}, maxSliceLen)
}

// RegisterCustomType is used to register custom types that may be
// unmarshaled into an interface
// [val] is a value of the type being registered
func (c *caminoLinearCodec) RegisterCustomType(val interface{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	valType := reflect.TypeOf(val)
	if c.registeredTypes.HasValue(valType) {
		return fmt.Errorf("%w: %v", codec.ErrDuplicateType, valType)
	}
	c.registeredTypes.Put(c.nextCustomTypeID, valType)
	c.nextCustomTypeID++
	return nil
}

// Skip some number of type IDs
func (c *caminoLinearCodec) SkipCustomRegistrations(num int) {
	c.lock.Lock()
	c.nextCustomTypeID += uint32(num)
	c.lock.Unlock()
}
