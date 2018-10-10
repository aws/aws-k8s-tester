package prow

import (
	"reflect"
	"sort"
	"testing"
)

func TestJobsSort(t *testing.T) {
	ss1 := []Job{
		{Type: TypePeriodic, Category: "5", Group: "c", ID: "1"},
		{Type: TypePeriodic, Category: "5", Group: "a", ID: "2"},
		{Type: TypePeriodic, Category: "5", Group: "a", ID: "1"},
		{Type: TypePostsubmit, Category: "5", Group: "a", ID: "2"},
		{Type: TypePostsubmit, Category: "5", Group: "a", ID: "1"},
		{Type: TypePostsubmit, Category: "3", Group: "a", ID: "1"},
		{Type: TypePresubmit, Category: "2"},
		{Type: TypePresubmit, Category: "1"},
	}
	ss2 := []Job{
		{Type: TypePresubmit, Category: "1"},
		{Type: TypePresubmit, Category: "2"},
		{Type: TypePostsubmit, Category: "3", Group: "a", ID: "1"},
		{Type: TypePostsubmit, Category: "5", Group: "a", ID: "1"},
		{Type: TypePostsubmit, Category: "5", Group: "a", ID: "2"},
		{Type: TypePeriodic, Category: "5", Group: "a", ID: "1"},
		{Type: TypePeriodic, Category: "5", Group: "c", ID: "1"},
		{Type: TypePeriodic, Category: "5", Group: "a", ID: "2"},
	}
	sort.Sort(Jobs(ss1))
	if !reflect.DeepEqual(ss1, ss2) {
		t.Fatalf("expected %+v, got %+v", ss2, ss1)
	}
}
