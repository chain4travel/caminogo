// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package avm

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAssetDescriptionC4T(t *testing.T) {
	env := setup(t, &envConfig{})
	env.vm.ctx.Lock.Unlock()
	defer stopEnv(t, env)

	type args struct {
		in0   *http.Request
		args  *GetAssetDescriptionArgs
		reply *GetAssetDescriptionReply
	}
	tests := []struct {
		name        string
		args        args
		expectedErr error
		want        []string
	}{
		{
			name: "With given assetId",
			args: args{
				in0:   nil,
				reply: &GetAssetDescriptionReply{},
				args: &GetAssetDescriptionArgs{
					AssetID: env.vm.ctx.AVAXAssetID.String(),
				},
			},
			want: []string{"AVAX", "SYMB", env.vm.ctx.AVAXAssetID.String()},
		},
		{
			name: "Without assetId",
			args: args{
				in0:   nil,
				reply: &GetAssetDescriptionReply{},
				args: &GetAssetDescriptionArgs{
					AssetID: env.vm.ctx.AVAXAssetID.String(),
				},
			},
			want: []string{"AVAX", "SYMB", env.vm.feeAssetID.String()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := env.service.GetAssetDescription(tt.args.in0, tt.args.args, tt.args.reply)
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.want[0], tt.args.reply.Name, "Wrong name returned from GetAssetDescription %s", tt.args.reply.Name)
			require.Equal(t, tt.want[1], tt.args.reply.Symbol, "Wrong symbol returned from GetAssetDescription %s", tt.args.reply.Symbol)
			require.Equal(t, tt.want[2], tt.args.reply.AssetID.String())
		})
	}
}

func stopEnv(t *testing.T, env *environment) {
	env.vm.ctx.Lock.Lock()
	require.NoError(t, env.vm.Shutdown(context.Background()))
	env.vm.ctx.Lock.Unlock()
}
