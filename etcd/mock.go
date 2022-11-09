// +build unit

package etcd

import (
	"context"
	"fmt"

	"github.com/coreos/etcd/clientv3"
)

type MockLease struct {
	Ttl                 int64
	LeaseResponse       *clientv3.LeaseGrantResponse
	LeaseRevokeResponse *clientv3.LeaseRevokeResponse
	Err                 error
}

type MockTxn struct {
	clientv3.TxnResponse
	OpsKeys   []string
	OpsValues []string
	Err       error
}

type MockKV struct {
	GetKey          string
	PutResponse     *clientv3.PutResponse
	CompactResponse *clientv3.CompactResponse
	OpResponse      *clientv3.OpResponse
	GetResponse     *clientv3.GetResponse
	DeleteResponse  *clientv3.DeleteResponse
	Mt              MockTxn
	Err             error
}

func (l *MockLease) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	if ttl != l.Ttl {
		return nil, fmt.Errorf("Expected ttl = %d, got ttl = %d", l.Ttl, ttl)
	}
	return l.LeaseResponse, l.Err
}

func (l *MockLease) Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	return l.LeaseRevokeResponse, l.Err
}

func (l *MockLease) TimeToLive(ctx context.Context, id clientv3.LeaseID, opts ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	return nil, nil
}

func (l *MockLease) Leases(ctx context.Context) (*clientv3.LeaseLeasesResponse, error) {
	return nil, nil
}

func (l *MockLease) KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	return nil, nil
}

func (l *MockLease) KeepAliveOnce(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
	return nil, nil
}

func (l *MockLease) Close() error {
	return nil
}

func (mt MockTxn) If(cs ...clientv3.Cmp) clientv3.Txn {
	return mt
}

func (mt MockTxn) Then(ops ...clientv3.Op) clientv3.Txn {
	if len(ops) != len(mt.OpsKeys) {
		mt.Err = fmt.Errorf("Expected %d length of ops, got %d", len(mt.OpsKeys), len(ops))
		return mt
	}

	for i := 0; i < len(ops); i++ {
		if string(ops[i].KeyBytes()) != mt.OpsKeys[i] {
			fmt.Println(string(ops[i].KeyBytes()), ":", mt.OpsKeys[i])
			mt.Err = fmt.Errorf("Expected opsKey = %s , got opsKey = %s", mt.OpsKeys[i], ops[i].KeyBytes())
			return mt
		}

		if string(ops[i].ValueBytes()) != mt.OpsValues[i] {
			fmt.Println(string(ops[i].ValueBytes()), ":", mt.OpsValues[i])
			mt.Err = fmt.Errorf("Expected opsValue = %s , got opsValue = %s", mt.OpsValues[i], ops[i].ValueBytes())
			return mt
		}

	}

	return mt
}

func (mt MockTxn) Else(ops ...clientv3.Op) clientv3.Txn {
	return mt
}

func (mt MockTxn) Commit() (*clientv3.TxnResponse, error) {
	return &mt.TxnResponse, mt.Err
}

func (kv *MockKV) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return kv.PutResponse, kv.Err
}

func (kv *MockKV) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return kv.CompactResponse, kv.Err
}

func (kv *MockKV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return kv.DeleteResponse, kv.Err
}

func (kv *MockKV) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return *kv.OpResponse, kv.Err
}

func (kv *MockKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if key != kv.GetKey {
		return nil, fmt.Errorf("Expected getKey = %s, got key = %s", kv.GetKey, key)
	}
	return kv.GetResponse, nil
}

func (kv *MockKV) Txn(ctx context.Context) clientv3.Txn {
	return kv.Mt
}
