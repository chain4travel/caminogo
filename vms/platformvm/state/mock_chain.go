// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/avalanchego/vms/platformvm/state (interfaces: Chain)

// Package state is a generated GoMock package.
package state

import (
	reflect "reflect"
	time "time"

	ids "github.com/ava-labs/avalanchego/ids"
	set "github.com/ava-labs/avalanchego/utils/set"
	avax "github.com/ava-labs/avalanchego/vms/components/avax"
	multisig "github.com/ava-labs/avalanchego/vms/components/multisig"
	config "github.com/ava-labs/avalanchego/vms/platformvm/config"
	deposit "github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	locked "github.com/ava-labs/avalanchego/vms/platformvm/locked"
	status "github.com/ava-labs/avalanchego/vms/platformvm/status"
	txs "github.com/ava-labs/avalanchego/vms/platformvm/txs"
	gomock "github.com/golang/mock/gomock"
)

// MockChain is a mock of Chain interface.
type MockChain struct {
	ctrl     *gomock.Controller
	recorder *MockChainMockRecorder
}

// MockChainMockRecorder is the mock recorder for MockChain.
type MockChainMockRecorder struct {
	mock *MockChain
}

// NewMockChain creates a new mock instance.
func NewMockChain(ctrl *gomock.Controller) *MockChain {
	mock := &MockChain{ctrl: ctrl}
	mock.recorder = &MockChainMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockChain) EXPECT() *MockChainMockRecorder {
	return m.recorder
}

// AddChain mocks base method.
func (m *MockChain) AddChain(arg0 *txs.Tx) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddChain", arg0)
}

// AddChain indicates an expected call of AddChain.
func (mr *MockChainMockRecorder) AddChain(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddChain", reflect.TypeOf((*MockChain)(nil).AddChain), arg0)
}

// SetDepositOffer mocks base method.
func (m *MockChain) SetDepositOffer(arg0 *deposit.Offer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetDepositOffer", arg0)
}

// SetDepositOffer indicates an expected call of SetDepositOffer.
func (mr *MockChainMockRecorder) SetDepositOffer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDepositOffer", reflect.TypeOf((*MockChain)(nil).SetDepositOffer), arg0)
}

// AddRewardUTXO mocks base method.
func (m *MockChain) AddRewardUTXO(arg0 ids.ID, arg1 *avax.UTXO) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddRewardUTXO", arg0, arg1)
}

// AddRewardUTXO indicates an expected call of AddRewardUTXO.
func (mr *MockChainMockRecorder) AddRewardUTXO(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRewardUTXO", reflect.TypeOf((*MockChain)(nil).AddRewardUTXO), arg0, arg1)
}

// AddSubnet mocks base method.
func (m *MockChain) AddSubnet(arg0 *txs.Tx) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddSubnet", arg0)
}

// AddSubnet indicates an expected call of AddSubnet.
func (mr *MockChainMockRecorder) AddSubnet(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSubnet", reflect.TypeOf((*MockChain)(nil).AddSubnet), arg0)
}

// AddSubnetTransformation mocks base method.
func (m *MockChain) AddSubnetTransformation(arg0 *txs.Tx) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddSubnetTransformation", arg0)
}

// AddSubnetTransformation indicates an expected call of AddSubnetTransformation.
func (mr *MockChainMockRecorder) AddSubnetTransformation(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSubnetTransformation", reflect.TypeOf((*MockChain)(nil).AddSubnetTransformation), arg0)
}

// AddTx mocks base method.
func (m *MockChain) AddTx(arg0 *txs.Tx, arg1 status.Status) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddTx", arg0, arg1)
}

// AddTx indicates an expected call of AddTx.
func (mr *MockChainMockRecorder) AddTx(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTx", reflect.TypeOf((*MockChain)(nil).AddTx), arg0, arg1)
}

// AddUTXO mocks base method.
func (m *MockChain) AddUTXO(arg0 *avax.UTXO) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddUTXO", arg0)
}

// AddUTXO indicates an expected call of AddUTXO.
func (mr *MockChainMockRecorder) AddUTXO(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddUTXO", reflect.TypeOf((*MockChain)(nil).AddUTXO), arg0)
}

// CaminoConfig mocks base method.
func (m *MockChain) CaminoConfig() (*CaminoConfig, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CaminoConfig")
	ret0, _ := ret[0].(*CaminoConfig)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CaminoConfig indicates an expected call of CaminoConfig.
func (mr *MockChainMockRecorder) CaminoConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CaminoConfig", reflect.TypeOf((*MockChain)(nil).CaminoConfig))
}

// Config mocks base method.
func (m *MockChain) Config() (*config.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Config")
	ret0, _ := ret[0].(*config.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Config indicates an expected call of Config.
func (mr *MockChainMockRecorder) Config() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Config", reflect.TypeOf((*MockChain)(nil).Config))
}

// DeleteCurrentDelegator mocks base method.
func (m *MockChain) DeleteCurrentDelegator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteCurrentDelegator", arg0)
}

// DeleteCurrentDelegator indicates an expected call of DeleteCurrentDelegator.
func (mr *MockChainMockRecorder) DeleteCurrentDelegator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCurrentDelegator", reflect.TypeOf((*MockChain)(nil).DeleteCurrentDelegator), arg0)
}

// DeleteCurrentValidator mocks base method.
func (m *MockChain) DeleteCurrentValidator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteCurrentValidator", arg0)
}

// DeleteCurrentValidator indicates an expected call of DeleteCurrentValidator.
func (mr *MockChainMockRecorder) DeleteCurrentValidator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCurrentValidator", reflect.TypeOf((*MockChain)(nil).DeleteCurrentValidator), arg0)
}

// DeletePendingDelegator mocks base method.
func (m *MockChain) DeletePendingDelegator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeletePendingDelegator", arg0)
}

// DeletePendingDelegator indicates an expected call of DeletePendingDelegator.
func (mr *MockChainMockRecorder) DeletePendingDelegator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePendingDelegator", reflect.TypeOf((*MockChain)(nil).DeletePendingDelegator), arg0)
}

// DeletePendingValidator mocks base method.
func (m *MockChain) DeletePendingValidator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeletePendingValidator", arg0)
}

// DeletePendingValidator indicates an expected call of DeletePendingValidator.
func (mr *MockChainMockRecorder) DeletePendingValidator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePendingValidator", reflect.TypeOf((*MockChain)(nil).DeletePendingValidator), arg0)
}

// DeleteUTXO mocks base method.
func (m *MockChain) DeleteUTXO(arg0 ids.ID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteUTXO", arg0)
}

// DeleteUTXO indicates an expected call of DeleteUTXO.
func (mr *MockChainMockRecorder) DeleteUTXO(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteUTXO", reflect.TypeOf((*MockChain)(nil).DeleteUTXO), arg0)
}

// GetAddressStates mocks base method.
func (m *MockChain) GetAddressStates(arg0 ids.ShortID) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAddressStates", arg0)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAddressStates indicates an expected call of GetAddressStates.
func (mr *MockChainMockRecorder) GetAddressStates(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAddressStates", reflect.TypeOf((*MockChain)(nil).GetAddressStates), arg0)
}

// GetAllDepositOffers mocks base method.
func (m *MockChain) GetAllDepositOffers() ([]*deposit.Offer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllDepositOffers")
	ret0, _ := ret[0].([]*deposit.Offer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllDepositOffers indicates an expected call of GetAllDepositOffers.
func (mr *MockChainMockRecorder) GetAllDepositOffers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllDepositOffers", reflect.TypeOf((*MockChain)(nil).GetAllDepositOffers))
}

// GetChains mocks base method.
func (m *MockChain) GetChains(arg0 ids.ID) ([]*txs.Tx, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChains", arg0)
	ret0, _ := ret[0].([]*txs.Tx)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChains indicates an expected call of GetChains.
func (mr *MockChainMockRecorder) GetChains(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChains", reflect.TypeOf((*MockChain)(nil).GetChains), arg0)
}

// GetClaimable mocks base method.
func (m *MockChain) GetClaimable(arg0 ids.ID) (*Claimable, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClaimable", arg0)
	ret0, _ := ret[0].(*Claimable)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClaimable indicates an expected call of GetClaimable.
func (mr *MockChainMockRecorder) GetClaimable(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClaimable", reflect.TypeOf((*MockChain)(nil).GetClaimable), arg0)
}

// GetCurrentDelegatorIterator mocks base method.
func (m *MockChain) GetCurrentDelegatorIterator(arg0 ids.ID, arg1 ids.NodeID) (StakerIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCurrentDelegatorIterator", arg0, arg1)
	ret0, _ := ret[0].(StakerIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCurrentDelegatorIterator indicates an expected call of GetCurrentDelegatorIterator.
func (mr *MockChainMockRecorder) GetCurrentDelegatorIterator(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCurrentDelegatorIterator", reflect.TypeOf((*MockChain)(nil).GetCurrentDelegatorIterator), arg0, arg1)
}

// GetCurrentStakerIterator mocks base method.
func (m *MockChain) GetCurrentStakerIterator() (StakerIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCurrentStakerIterator")
	ret0, _ := ret[0].(StakerIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCurrentStakerIterator indicates an expected call of GetCurrentStakerIterator.
func (mr *MockChainMockRecorder) GetCurrentStakerIterator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCurrentStakerIterator", reflect.TypeOf((*MockChain)(nil).GetCurrentStakerIterator))
}

// GetCurrentSupply mocks base method.
func (m *MockChain) GetCurrentSupply(arg0 ids.ID) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCurrentSupply", arg0)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCurrentSupply indicates an expected call of GetCurrentSupply.
func (mr *MockChainMockRecorder) GetCurrentSupply(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCurrentSupply", reflect.TypeOf((*MockChain)(nil).GetCurrentSupply), arg0)
}

// GetCurrentValidator mocks base method.
func (m *MockChain) GetCurrentValidator(arg0 ids.ID, arg1 ids.NodeID) (*Staker, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCurrentValidator", arg0, arg1)
	ret0, _ := ret[0].(*Staker)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCurrentValidator indicates an expected call of GetCurrentValidator.
func (mr *MockChainMockRecorder) GetCurrentValidator(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCurrentValidator", reflect.TypeOf((*MockChain)(nil).GetCurrentValidator), arg0, arg1)
}

// GetDeposit mocks base method.
func (m *MockChain) GetDeposit(arg0 ids.ID) (*deposit.Deposit, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeposit", arg0)
	ret0, _ := ret[0].(*deposit.Deposit)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeposit indicates an expected call of GetDeposit.
func (mr *MockChainMockRecorder) GetDeposit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeposit", reflect.TypeOf((*MockChain)(nil).GetDeposit), arg0)
}

// GetNextToUnlockDepositTime mocks base method.
func (m *MockChain) GetNextToUnlockDepositTime(arg0 set.Set[ids.ID]) (time.Time, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNextToUnlockDepositTime", arg0)
	ret0, _ := ret[0].(time.Time)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNextToUnlockDepositTime indicates an expected call of GetNextToUnlockDepositTime.
func (mr *MockChainMockRecorder) GetNextToUnlockDepositTime(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNextToUnlockDepositTime", reflect.TypeOf((*MockChain)(nil).GetNextToUnlockDepositTime), arg0)
}

// GetNextToUnlockDepositIDsAndTime mocks base method.
func (m *MockChain) GetNextToUnlockDepositIDsAndTime(arg0 set.Set[ids.ID]) ([]ids.ID, time.Time, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNextToUnlockDepositIDsAndTime", arg0)
	ret0, _ := ret[0].([]ids.ID)
	ret1, _ := ret[1].(time.Time)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetNextToUnlockDepositIDsAndTime indicates an expected call of GetNextToUnlockDepositIDsAndTime.
func (mr *MockChainMockRecorder) GetNextToUnlockDepositIDsAndTime(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNextToUnlockDepositIDsAndTime", reflect.TypeOf((*MockChain)(nil).GetNextToUnlockDepositIDsAndTime), arg0)
}

// GetDepositOffer mocks base method.
func (m *MockChain) GetDepositOffer(arg0 ids.ID) (*deposit.Offer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDepositOffer", arg0)
	ret0, _ := ret[0].(*deposit.Offer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDepositOffer indicates an expected call of GetDepositOffer.
func (mr *MockChainMockRecorder) GetDepositOffer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDepositOffer", reflect.TypeOf((*MockChain)(nil).GetDepositOffer), arg0)
}

// GetMultisigAlias mocks base method.
func (m *MockChain) GetMultisigAlias(arg0 ids.ShortID) (*multisig.AliasWithNonce, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMultisigAlias", arg0)
	ret0, _ := ret[0].(*multisig.AliasWithNonce)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMultisigAlias indicates an expected call of GetMultisigAlias.
func (mr *MockChainMockRecorder) GetMultisigAlias(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMultisigAlias", reflect.TypeOf((*MockChain)(nil).GetMultisigAlias), arg0)
}

// GetNotDistributedValidatorReward mocks base method.
func (m *MockChain) GetNotDistributedValidatorReward() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNotDistributedValidatorReward")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNotDistributedValidatorReward indicates an expected call of GetNotDistributedValidatorReward.
func (mr *MockChainMockRecorder) GetNotDistributedValidatorReward() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNotDistributedValidatorReward", reflect.TypeOf((*MockChain)(nil).GetNotDistributedValidatorReward))
}

// GetPendingDelegatorIterator mocks base method.
func (m *MockChain) GetPendingDelegatorIterator(arg0 ids.ID, arg1 ids.NodeID) (StakerIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPendingDelegatorIterator", arg0, arg1)
	ret0, _ := ret[0].(StakerIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPendingDelegatorIterator indicates an expected call of GetPendingDelegatorIterator.
func (mr *MockChainMockRecorder) GetPendingDelegatorIterator(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPendingDelegatorIterator", reflect.TypeOf((*MockChain)(nil).GetPendingDelegatorIterator), arg0, arg1)
}

// GetPendingStakerIterator mocks base method.
func (m *MockChain) GetPendingStakerIterator() (StakerIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPendingStakerIterator")
	ret0, _ := ret[0].(StakerIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPendingStakerIterator indicates an expected call of GetPendingStakerIterator.
func (mr *MockChainMockRecorder) GetPendingStakerIterator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPendingStakerIterator", reflect.TypeOf((*MockChain)(nil).GetPendingStakerIterator))
}

// GetPendingValidator mocks base method.
func (m *MockChain) GetPendingValidator(arg0 ids.ID, arg1 ids.NodeID) (*Staker, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPendingValidator", arg0, arg1)
	ret0, _ := ret[0].(*Staker)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPendingValidator indicates an expected call of GetPendingValidator.
func (mr *MockChainMockRecorder) GetPendingValidator(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPendingValidator", reflect.TypeOf((*MockChain)(nil).GetPendingValidator), arg0, arg1)
}

// GetDeferredStakerIterator mocks base method.
func (m *MockChain) GetDeferredStakerIterator() (StakerIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeferredStakerIterator")
	ret0, _ := ret[0].(StakerIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeferredStakerIterator indicates an expected call of GetDeferredStakerIterator.
func (mr *MockChainMockRecorder) GetDeferredStakerIterator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeferredStakerIterator", reflect.TypeOf((*MockChain)(nil).GetDeferredStakerIterator))
}

// GetDeferredValidator mocks base method.
func (m *MockChain) GetDeferredValidator(arg0 ids.ID, arg1 ids.NodeID) (*Staker, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeferredValidator", arg0, arg1)
	ret0, _ := ret[0].(*Staker)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeferredValidator indicates an expected call of GetDeferredValidator.
func (mr *MockChainMockRecorder) GetDeferredValidator(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeferredValidator", reflect.TypeOf((*MockChain)(nil).GetDeferredValidator), arg0, arg1)
}

// DeleteDeferredValidator mocks base method.
func (m *MockChain) DeleteDeferredValidator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteDeferredValidator", arg0)
}

// DeleteDeferredValidator indicates an expected call of DeleteDeferredValidator.
func (mr *MockChainMockRecorder) DeleteDeferredValidator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDeferredValidator", reflect.TypeOf((*MockChain)(nil).DeleteDeferredValidator), arg0)
}

// PutDeferredValidator mocks base method.
func (m *MockChain) PutDeferredValidator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PutDeferredValidator", arg0)
}

// PutDeferredValidator indicates an expected call of PutDeferredValidator.
func (mr *MockChainMockRecorder) PutDeferredValidator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutDeferredValidator", reflect.TypeOf((*MockChain)(nil).PutDeferredValidator), arg0)
}

// GetRewardUTXOs mocks base method.
func (m *MockChain) GetRewardUTXOs(arg0 ids.ID) ([]*avax.UTXO, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRewardUTXOs", arg0)
	ret0, _ := ret[0].([]*avax.UTXO)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRewardUTXOs indicates an expected call of GetRewardUTXOs.
func (mr *MockChainMockRecorder) GetRewardUTXOs(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRewardUTXOs", reflect.TypeOf((*MockChain)(nil).GetRewardUTXOs), arg0)
}

// GetShortIDLink mocks base method.
func (m *MockChain) GetShortIDLink(arg0 ids.ShortID, arg1 ShortLinkKey) (ids.ShortID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetShortIDLink", arg0, arg1)
	ret0, _ := ret[0].(ids.ShortID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetShortIDLink indicates an expected call of GetShortIDLink.
func (mr *MockChainMockRecorder) GetShortIDLink(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetShortIDLink", reflect.TypeOf((*MockChain)(nil).GetShortIDLink), arg0, arg1)
}

// GetSubnetTransformation mocks base method.
func (m *MockChain) GetSubnetTransformation(arg0 ids.ID) (*txs.Tx, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubnetTransformation", arg0)
	ret0, _ := ret[0].(*txs.Tx)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSubnetTransformation indicates an expected call of GetSubnetTransformation.
func (mr *MockChainMockRecorder) GetSubnetTransformation(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubnetTransformation", reflect.TypeOf((*MockChain)(nil).GetSubnetTransformation), arg0)
}

// GetSubnets mocks base method.
func (m *MockChain) GetSubnets() ([]*txs.Tx, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubnets")
	ret0, _ := ret[0].([]*txs.Tx)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSubnets indicates an expected call of GetSubnets.
func (mr *MockChainMockRecorder) GetSubnets() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubnets", reflect.TypeOf((*MockChain)(nil).GetSubnets))
}

// GetTimestamp mocks base method.
func (m *MockChain) GetTimestamp() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTimestamp")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

// GetTimestamp indicates an expected call of GetTimestamp.
func (mr *MockChainMockRecorder) GetTimestamp() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTimestamp", reflect.TypeOf((*MockChain)(nil).GetTimestamp))
}

// GetTx mocks base method.
func (m *MockChain) GetTx(arg0 ids.ID) (*txs.Tx, status.Status, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTx", arg0)
	ret0, _ := ret[0].(*txs.Tx)
	ret1, _ := ret[1].(status.Status)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetTx indicates an expected call of GetTx.
func (mr *MockChainMockRecorder) GetTx(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTx", reflect.TypeOf((*MockChain)(nil).GetTx), arg0)
}

// GetUTXO mocks base method.
func (m *MockChain) GetUTXO(arg0 ids.ID) (*avax.UTXO, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUTXO", arg0)
	ret0, _ := ret[0].(*avax.UTXO)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUTXO indicates an expected call of GetUTXO.
func (mr *MockChainMockRecorder) GetUTXO(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUTXO", reflect.TypeOf((*MockChain)(nil).GetUTXO), arg0)
}

// LockedUTXOs mocks base method.
func (m *MockChain) LockedUTXOs(arg0 set.Set[ids.ID], arg1 set.Set[ids.ShortID], arg2 locked.State) ([]*avax.UTXO, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LockedUTXOs", arg0, arg1, arg2)
	ret0, _ := ret[0].([]*avax.UTXO)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LockedUTXOs indicates an expected call of LockedUTXOs.
func (mr *MockChainMockRecorder) LockedUTXOs(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LockedUTXOs", reflect.TypeOf((*MockChain)(nil).LockedUTXOs), arg0, arg1, arg2)
}

// PutCurrentDelegator mocks base method.
func (m *MockChain) PutCurrentDelegator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PutCurrentDelegator", arg0)
}

// PutCurrentDelegator indicates an expected call of PutCurrentDelegator.
func (mr *MockChainMockRecorder) PutCurrentDelegator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutCurrentDelegator", reflect.TypeOf((*MockChain)(nil).PutCurrentDelegator), arg0)
}

// PutCurrentValidator mocks base method.
func (m *MockChain) PutCurrentValidator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PutCurrentValidator", arg0)
}

// PutCurrentValidator indicates an expected call of PutCurrentValidator.
func (mr *MockChainMockRecorder) PutCurrentValidator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutCurrentValidator", reflect.TypeOf((*MockChain)(nil).PutCurrentValidator), arg0)
}

// PutPendingDelegator mocks base method.
func (m *MockChain) PutPendingDelegator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PutPendingDelegator", arg0)
}

// PutPendingDelegator indicates an expected call of PutPendingDelegator.
func (mr *MockChainMockRecorder) PutPendingDelegator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutPendingDelegator", reflect.TypeOf((*MockChain)(nil).PutPendingDelegator), arg0)
}

// PutPendingValidator mocks base method.
func (m *MockChain) PutPendingValidator(arg0 *Staker) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PutPendingValidator", arg0)
}

// PutPendingValidator indicates an expected call of PutPendingValidator.
func (mr *MockChainMockRecorder) PutPendingValidator(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutPendingValidator", reflect.TypeOf((*MockChain)(nil).PutPendingValidator), arg0)
}

// SetAddressStates mocks base method.
func (m *MockChain) SetAddressStates(arg0 ids.ShortID, arg1 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetAddressStates", arg0, arg1)
}

// SetAddressStates indicates an expected call of SetAddressStates.
func (mr *MockChainMockRecorder) SetAddressStates(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAddressStates", reflect.TypeOf((*MockChain)(nil).SetAddressStates), arg0, arg1)
}

// SetClaimable mocks base method.
func (m *MockChain) SetClaimable(arg0 ids.ID, arg1 *Claimable) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetClaimable", arg0, arg1)
}

// SetClaimable indicates an expected call of SetClaimable.
func (mr *MockChainMockRecorder) SetClaimable(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetClaimable", reflect.TypeOf((*MockChain)(nil).SetClaimable), arg0, arg1)
}

// SetCurrentSupply mocks base method.
func (m *MockChain) SetCurrentSupply(arg0 ids.ID, arg1 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetCurrentSupply", arg0, arg1)
}

// SetCurrentSupply indicates an expected call of SetCurrentSupply.
func (mr *MockChainMockRecorder) SetCurrentSupply(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCurrentSupply", reflect.TypeOf((*MockChain)(nil).SetCurrentSupply), arg0, arg1)
}

// SetLastRewardImportTimestamp mocks base method.
func (m *MockChain) SetLastRewardImportTimestamp(arg0 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetLastRewardImportTimestamp", arg0)
}

// SetLastRewardImportTimestamp indicates an expected call of SetLastRewardImportTimestamp.
func (mr *MockChainMockRecorder) SetLastRewardImportTimestamp(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLastRewardImportTimestamp", reflect.TypeOf((*MockChain)(nil).SetLastRewardImportTimestamp),
		arg0)
}

// SetMultisigAlias mocks base method.
func (m *MockChain) SetMultisigAlias(arg0 *multisig.AliasWithNonce) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetMultisigAlias", arg0)
}

// SetMultisigAlias indicates an expected call of SetMultisigAlias.
func (mr *MockChainMockRecorder) SetMultisigAlias(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetMultisigAlias", reflect.TypeOf((*MockChain)(nil).SetMultisigAlias), arg0)
}

// SetNotDistributedValidatorReward mocks base method.
func (m *MockChain) SetNotDistributedValidatorReward(arg0 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetNotDistributedValidatorReward", arg0)
}

// SetNotDistributedValidatorReward indicates an expected call of SetNotDistributedValidatorReward.
func (mr *MockChainMockRecorder) SetNotDistributedValidatorReward(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNotDistributedValidatorReward", reflect.TypeOf((*MockChain)(nil).SetNotDistributedValidatorReward), arg0)
}

// SetShortIDLink mocks base method.
func (m *MockChain) SetShortIDLink(arg0 ids.ShortID, arg1 ShortLinkKey, arg2 *ids.ShortID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetShortIDLink", arg0, arg1, arg2)
}

// SetShortIDLink indicates an expected call of SetShortIDLink.
func (mr *MockChainMockRecorder) SetShortIDLink(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetShortIDLink", reflect.TypeOf((*MockChain)(nil).SetShortIDLink), arg0, arg1, arg2)
}

// SetTimestamp mocks base method.
func (m *MockChain) SetTimestamp(arg0 time.Time) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetTimestamp", arg0)
}

// SetTimestamp indicates an expected call of SetTimestamp.
func (mr *MockChainMockRecorder) SetTimestamp(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTimestamp", reflect.TypeOf((*MockChain)(nil).SetTimestamp), arg0)
}

// AddDeposit mocks base method.
func (m *MockChain) AddDeposit(arg0 ids.ID, arg1 *deposit.Deposit) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddDeposit", arg0, arg1)
}

// AddDeposit indicates an expected call of AddDeposit.
func (mr *MockChainMockRecorder) AddDeposit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddDeposit", reflect.TypeOf((*MockChain)(nil).AddDeposit), arg0, arg1)
}

// ModifyDeposit mocks base method.
func (m *MockChain) ModifyDeposit(arg0 ids.ID, arg1 *deposit.Deposit) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ModifyDeposit", arg0, arg1)
}

// ModifyDeposit indicates an expected call of ModifyDeposit.
func (mr *MockChainMockRecorder) ModifyDeposit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ModifyDeposit", reflect.TypeOf((*MockChain)(nil).ModifyDeposit), arg0, arg1)
}

// RemoveDeposit mocks base method.
func (m *MockChain) RemoveDeposit(arg0 ids.ID, arg1 *deposit.Deposit) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RemoveDeposit", arg0, arg1)
}

// RemoveDeposit indicates an expected call of RemoveDeposit.
func (mr *MockChainMockRecorder) RemoveDeposit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveDeposit", reflect.TypeOf((*MockChain)(nil).RemoveDeposit), arg0, arg1)
}
