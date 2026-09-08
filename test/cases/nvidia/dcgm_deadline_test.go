package nvidia

import "testing"

func TestDcgmDiagDeadlineSeconds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		minutes int
		want    int
		wantErr bool
	}{
		{name: "zero is rejected", minutes: 0, wantErr: true},
		{name: "negative is rejected", minutes: -5, wantErr: true},
		{name: "one minute would render zero, so is rejected", minutes: 1, wantErr: true},
		{name: "two minutes is the smallest accepted", minutes: 2, want: 60},
		{name: "the default", minutes: 180, want: 10740},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dcgmDiagDeadlineSeconds(tc.minutes)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("dcgmDiagDeadlineSeconds(%d) = %d, want an error", tc.minutes, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("dcgmDiagDeadlineSeconds(%d) returned unexpected error: %v", tc.minutes, err)
			}
			if got != tc.want {
				t.Errorf("dcgmDiagDeadlineSeconds(%d) = %d, want %d", tc.minutes, got, tc.want)
			}
		})
	}
}

// TestDcgmDiagDeadlineAlwaysPositive asserts the property the Job depends on
// across the whole accepted range, rather than only at the values enumerated
// above: anything the bound admits must render a deadline Kubernetes accepts.
func TestDcgmDiagDeadlineAlwaysPositive(t *testing.T) {
	for m := minDcgmDiagTimeoutMinutes; m <= 600; m++ {
		got, err := dcgmDiagDeadlineSeconds(m)
		if err != nil {
			t.Fatalf("dcgmDiagDeadlineSeconds(%d) rejected a value inside the accepted range: %v", m, err)
		}
		if got <= 0 {
			t.Fatalf("dcgmDiagDeadlineSeconds(%d) = %d, must be positive", m, got)
		}
	}
}
