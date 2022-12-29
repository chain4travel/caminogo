// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/avalanchego/vms/platformvm/state (interfaces: Chain)

// Package state is a generated GoMock package.
package state

import (
	ids "github.com/ava-labs/avalanchego/ids"
	set "github.com/ava-labs/avalanchego/utils/set"
	avax "github.com/ava-labs/avalanchego/vms/components/avax"
	deposit "github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	locked "github.com/ava-labs/avalanchego/vms/platformvm/locked"
	status "github.com/ava-labs/avalanchego/vms/platformvm/status"
	txs "github.com/ava-labs/avalanchego/vms/platformvm/txs"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	time "time"
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

// AddDepositOffer mocks base method.
func (m *MockChain) AddDepositOffer(arg0 *deposit.Offer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddDepositOffer", arg0)
}

// AddDepositOffer indicates an expected call of AddDepositOffer.
func (mr *MockChainMockRecorder) AddDepositOffer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddDepositOffer", reflect.TypeOf((*MockChain)(nil).AddDepositOffer), arg0)
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

// GetMultisigOwner mocks base method.
func (m *MockChain) GetMultisigOwner(arg0 ids.ShortID) (*MultisigOwner, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMultisigOwner", arg0)
	ret0, _ := ret[0].(*MultisigOwner)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMultisigOwner indicates an expected call of GetMultisigOwner.
func (mr *MockChainMockRecorder) GetMultisigOwner(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMultisigOwner", reflect.TypeOf((*MockState)(nil).GetMultisigOwner), arg0)
}

// SetNodeConsortiumMember mocks base method.
func (m *MockChain) SetNodeConsortiumMember(arg0 ids.NodeID, arg1 ids.ShortID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetNodeConsortiumMember", arg0)
}

// SetNodeConsortiumMember indicates an expected call of SetNodeConsortiumMember.
func (mr *MockChainMockRecorder) SetNodeConsortiumMember(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNodeConsortiumMember", reflect.TypeOf((*MockState)(nil).SetNodeConsortiumMember), arg0, arg1)
}

// GetNodeConsortiumMember mocks base method.
func (m *MockChain) GetNodeConsortiumMember(arg0 ids.NodeID) (ids.ShortID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNodeConsortiumMember", arg0)
	ret0, _ := ret[0].(ids.ShortID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNodeConsortiumMember indicates an expected call of GetNodeConsortiumMember.
func (mr *MockChainMockRecorder) GetNodeConsortiumMember(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNodeConsortiumMember", reflect.TypeOf((*MockState)(nil).GetNodeConsortiumMember), arg0)
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

// SetMultisigOwner mocks base method.
func (m *MockChain) SetMultisigOwner(arg0 *MultisigOwner) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetMultisigOwner", arg0)
}

// SetMultisigOwner indicates an expected call of SetMultisigOwner.
func (mr *MockChainMockRecorder) SetMultisigOwner(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetMultisigOwner", reflect.TypeOf((*MockState)(nil).SetMultisigOwner), arg0)
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

// UpdateDeposit mocks base method.
func (m *MockChain) UpdateDeposit(arg0 ids.ID, arg1 *deposit.Deposit) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateDeposit", arg0, arg1)
}

// UpdateDeposit indicates an expected call of UpdateDeposit.
func (mr *MockChainMockRecorder) UpdateDeposit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDeposit", reflect.TypeOf((*MockChain)(nil).UpdateDeposit), arg0, arg1)
}
