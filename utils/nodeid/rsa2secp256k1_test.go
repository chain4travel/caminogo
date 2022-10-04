// Copyright (C) 2022, Chain4Travel AG. All rights reserved.

// See the file LICENSE for licensing terms.

package nodeid

import (
	x509 "crypto/x509"
	"reflect"
	"testing"
)

func TestRecoverSecp256PublicKey(t *testing.T) {
	type args struct {
		cert *x509.Certificate
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RecoverSecp256PublicKey(tt.args.cert)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecoverSecp256PublicKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RecoverSecp256PublicKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
