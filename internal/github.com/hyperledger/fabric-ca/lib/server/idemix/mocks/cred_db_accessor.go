// Code generated by mockery v2.7.4. DO NOT EDIT.

package mocks

import (
	db "hlf-easy/internal/github.com/hyperledger/fabric-ca/lib/server/db"
	idemix "hlf-easy/internal/github.com/hyperledger/fabric-ca/lib/server/idemix"

	mock "github.com/stretchr/testify/mock"
)

// CredDBAccessor is an autogenerated mock type for the CredDBAccessor type
type CredDBAccessor struct {
	mock.Mock
}

// GetCredential provides a mock function with given fields: revocationHandle
func (_m *CredDBAccessor) GetCredential(revocationHandle string) (*idemix.CredRecord, error) {
	ret := _m.Called(revocationHandle)

	var r0 *idemix.CredRecord
	if rf, ok := ret.Get(0).(func(string) *idemix.CredRecord); ok {
		r0 = rf(revocationHandle)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*idemix.CredRecord)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(revocationHandle)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCredentialsByID provides a mock function with given fields: id
func (_m *CredDBAccessor) GetCredentialsByID(id string) ([]idemix.CredRecord, error) {
	ret := _m.Called(id)

	var r0 []idemix.CredRecord
	if rf, ok := ret.Get(0).(func(string) []idemix.CredRecord); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]idemix.CredRecord)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRevokedCredentials provides a mock function with given fields:
func (_m *CredDBAccessor) GetRevokedCredentials() ([]idemix.CredRecord, error) {
	ret := _m.Called()

	var r0 []idemix.CredRecord
	if rf, ok := ret.Get(0).(func() []idemix.CredRecord); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]idemix.CredRecord)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// InsertCredential provides a mock function with given fields: cr
func (_m *CredDBAccessor) InsertCredential(cr idemix.CredRecord) error {
	ret := _m.Called(cr)

	var r0 error
	if rf, ok := ret.Get(0).(func(idemix.CredRecord) error); ok {
		r0 = rf(cr)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetDB provides a mock function with given fields: _a0
func (_m *CredDBAccessor) SetDB(_a0 db.FabricCADB) {
	_m.Called(_a0)
}
