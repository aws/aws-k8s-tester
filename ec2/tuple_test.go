package ec2

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func Test_byTime(t *testing.T) {
	ts := time.Time{}
	tss1 := []tupleTime{
		{ts: ts.Add(time.Second), name: "1"},
		{ts: ts.Add(2 * time.Second), name: "2"},
		{ts: ts.Add(3 * time.Second), name: "3"},
	}
	sort.Sort(sort.Reverse(tupleTimes(tss1)))
	tss2 := []tupleTime{
		{ts: ts.Add(3 * time.Second), name: "3"},
		{ts: ts.Add(2 * time.Second), name: "2"},
		{ts: ts.Add(time.Second), name: "1"},
	}
	if !reflect.DeepEqual(tss1, tss2) {
		t.Fatalf("expected %+v, got %+v", tss2, tss1)
	}
}
