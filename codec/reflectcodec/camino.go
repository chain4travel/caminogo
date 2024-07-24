// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package reflectcodec

import "reflect"

const (
	upgradeVersionTagName     = "upgradeVersion"
	UpgradeVersionIDFieldName = "UpgradeVersionID"
)

func checkUpgrade(t reflect.Type, numFields int) (bool, int) {
	if numFields > 0 &&
		t.Field(0).Type.Kind() == reflect.Uint64 &&
		t.Field(0).Name == UpgradeVersionIDFieldName {
		return true, 1
	}
	return false, 0
}

type SerializedFields struct {
	Fields            []FieldDesc
	CheckUpgrade      bool
	MaxUpgradeVersion uint16
}

type FieldDesc struct {
	Index          int
	UpgradeVersion uint16
}
