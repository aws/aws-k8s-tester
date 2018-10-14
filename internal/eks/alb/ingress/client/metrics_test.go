package client

import "testing"

func TestParse(t *testing.T) {
	// parse 'ingress_client_count_success{Endpoint="URL",Route="/ingress-test-00028"} 307'
	cm1 := parseCountMetrics(`ingress_client_count_success{Endpoint="URL",Route="/ingress-test-00028"} 307`)
	if cm1.Endpoint != "URL" {
		t.Fatalf("Endpoint expected 'URL', got %q", cm1.Endpoint)
	}
	if cm1.Route != "/ingress-test-00028" {
		t.Fatalf("Route expected '/ingress-test-00028', got %q", cm1.Route)
	}
	if cm1.count != 307 {
		t.Fatalf("count expected '307', got %d", cm1.count)
	}

	// parse 'ingress_client_count_failure{Endpoint="URL",Route="/ingress-test-00028"} 307'
	cm2 := parseCountMetrics(`ingress_client_count_failure{Endpoint="URL",Route="/ingress-test-00028"} 307`)
	if cm2.Endpoint != "URL" {
		t.Fatalf("Endpoint expected 'URL', got %q", cm2.Endpoint)
	}
	if cm2.Route != "/ingress-test-00028" {
		t.Fatalf("Route expected '/ingress-test-00028', got %q", cm2.Route)
	}
	if cm2.count != 307 {
		t.Fatalf("count expected '307', got %d", cm2.count)
	}

	// parse 'ingress_client_latency_bucket{Route="URL",To="/ingress-test-00001",le="0.0001"} 10'
	hm1 := parseHistogramMetrics(`ingress_client_latency_bucket{Route="URL",To="/ingress-test-00001",le="0.0001"} 10`)
	if hm1.Route != "URL" {
		t.Fatalf("Route expected 'URL', got %q", hm1.Route)
	}
	if hm1.To != "/ingress-test-00001" {
		t.Fatalf("To expected '/ingress-test-00001', got %q", hm1.To)
	}
	if hm1.lessEqualMs != 0.1 {
		t.Fatalf("lessEqualMs expected '0.1', got %f", hm1.lessEqualMs)
	}
	if hm1.count != 10 {
		t.Fatalf("count expected '10', got %d", hm1.count)
	}
}
