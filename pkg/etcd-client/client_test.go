package etcdclient

import (
	"fmt"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcd(t *testing.T) {
	// t.Skip()

	e, err := New(Config{
		EtcdClientConfig: clientv3.Config{
			Endpoints: []string{"localhost:2379"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = e.Put(5*time.Second, "a", "b", 15*time.Second)
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
