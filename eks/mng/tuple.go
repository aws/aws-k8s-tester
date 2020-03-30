package mng

import "time"

type tupleTime struct {
	ts   time.Time
	name string
}

type tupleTimes []tupleTime

func (ts tupleTimes) Len() int { return len(ts) }

func (ts tupleTimes) Less(i, j int) bool {
	return ts[j].ts.After(ts[i].ts)
}

func (ts tupleTimes) Swap(i, j int) {
	t := ts[i]
	ts[i] = ts[j]
	ts[j] = t
}
