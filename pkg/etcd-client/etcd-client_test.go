package etcdclient

import (
	"fmt"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
)

func TestEtcd(t *testing.T) {
	t.Skip()

	e, err := New(Config{
		EtcdClientConfig: clientv3.Config{
			Endpoints: []string{"localhost:2379"},
		},
		ListBath:     1,
		ListInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	ok, err := e.Campaign("hello", 5*time.Second)
	fmt.Println(ok, err)

	kvs, err := e.List("foo", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range kvs {
		fmt.Println(kv.String())
	}
}
