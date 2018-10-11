package wrk

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRun(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("OK"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	rs, err := Run(Config{
		Logger:        zap.NewExample(),
		StartAtMinute: 18,

		Threads:     1,
		Connections: 5,
		Duration:    5 * time.Second,
		Endpoint:    ts.URL,
	})
	if err != nil {
		t.Skip(err)
	}
	fmt.Printf("%+v\n", rs)
}

func TestParse(t *testing.T) {
	s1 := `Running 30s test @ http://6ecff19f-default-ingressfo-af34-486698562.us-west-2.elb.amazonaws.com
  2 threads and 200 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    18.65s     5.23s   27.56s    58.09%
    Req/Sec   515.00      1.00   516.00    100.00%
  30590 requests in 30.00s, 1.18GB read
Requests/sec:   1019.55
Transfer/sec:     40.14MB
`
	rs1, err := Parse(s1)
	if err != nil {
		t.Fatal(err)
	}
	if rs1.Threads != 2 {
		t.Fatalf("Threads expected 2, got %d", rs1.Threads)
	}
	if rs1.Connections != 200 {
		t.Fatalf("Connections expected 200, got %d", rs1.Connections)
	}
	if rs1.LatencyAvgMs != 18650.00 {
		t.Fatalf("LatencyAvg expected 18650ms, got %f", rs1.LatencyAvgMs)
	}
	if rs1.LatencyAvg != 18*time.Second+650*time.Millisecond {
		t.Fatalf("LatencyAvg expected 18.65s, got %v", rs1.LatencyAvg)
	}
	if rs1.LatencyStdev != 5*time.Second+230*time.Millisecond {
		t.Fatalf("LatencyStdev expected 5.23s, got %v", rs1.LatencyStdev)
	}
	if rs1.LatencyMax != 27*time.Second+560*time.Millisecond {
		t.Fatalf("LatencyMax expected 27.56s, got %v", rs1.LatencyMax)
	}
	if rs1.RequestsPerSecAvg != 515.00 {
		t.Fatalf("RequestsPerSecAvg expected 515.0, got %f", rs1.RequestsPerSecAvg)
	}
	if rs1.RequestsPerSecStdev != 1.00 {
		t.Fatalf("RequestsPerSecStdev expected 1.0, got %f", rs1.RequestsPerSecStdev)
	}
	if rs1.RequestsPerSecMax != 516.0 {
		t.Fatalf("RequestsPerSecMax expected 516.0, got %f", rs1.RequestsPerSecMax)
	}
	if rs1.TotalRequests != 30590 {
		t.Fatalf("TotalRequests expected 30590, got %d", rs1.TotalRequests)
	}
	if rs1.TotalTook != 30*time.Second {
		t.Fatalf("TotalRequests expected 30s, got %v", rs1.TotalTook)
	}
	if rs1.TotalReadDataBytes != 1180000000 {
		t.Fatalf("TotalReadDataBytes expected 1180000000, got %d", rs1.TotalReadDataBytes)
	}
	if rs1.RequestsPerSec != 1019.55 {
		t.Fatalf("RequestsPerSec expected 1019.55, got %f", rs1.RequestsPerSec)
	}
	if rs1.TransferPerSecBytes != 40140000 {
		t.Fatalf("TransferPerSecBytes expected 40140000, got %d", rs1.TransferPerSecBytes)
	}

	s2 := `Running 10s test @ http://127.0.0.1
2 threads and 300 connections
Thread Stats   Avg      Stdev     Max   +/- Stdev
  Latency   156.90ms  310.60ms   2.00s    86.36%
  Req/Sec     9.65k     4.62k   21.09k    65.77%
Latency Distribution
   50%    7.99ms
   75%  132.30ms
   90%  616.10ms
   99%    1.39s
148205 requests in 10.03s, 5.69GB read
Socket errors: connect 0, read 0, write 0, timeout 60
Requests/sec:  14775.76
Transfer/sec:    580.57MB
`
	rs2, err := Parse(s2)
	if err != nil {
		t.Fatal(err)
	}
	if rs2.Threads != 2 {
		t.Fatalf("Threads expected 2, got %d", rs2.Threads)
	}
	if rs2.Connections != 300 {
		t.Fatalf("Connections expected 200, got %d", rs2.Connections)
	}
	if rs2.LatencyAvg != 156*time.Millisecond+900*time.Microsecond {
		t.Fatalf("LatencyAvg expected 18.65s, got %v", rs2.LatencyAvg)
	}
	if rs2.LatencyStdev != 310*time.Millisecond+600*time.Microsecond {
		t.Fatalf("LatencyStdev expected 5.23s, got %v", rs2.LatencyStdev)
	}
	if rs2.LatencyMax != 2*time.Second {
		t.Fatalf("LatencyMax expected 27.56s, got %v", rs2.LatencyMax)
	}
	if rs2.RequestsPerSecAvg != 9650.0 {
		t.Fatalf("RequestsPerSecAvg expected 9650.0, got %f", rs2.RequestsPerSecAvg)
	}
	if rs2.RequestsPerSecStdev != 4620.0 {
		t.Fatalf("RequestsPerSecStdev expected 4620.0, got %f", rs2.RequestsPerSecStdev)
	}
	if rs2.RequestsPerSecMax != 21090.0 {
		t.Fatalf("RequestsPerSecMax expected 21090.0, got %f", rs2.RequestsPerSecMax)
	}
	if rs2.Latency50Pct != 7*time.Millisecond+990*time.Microsecond {
		t.Fatalf("Latency50Pct unexpected %v", rs2.Latency50Pct)
	}
	if rs2.Latency75Pct != 132*time.Millisecond+300*time.Microsecond {
		t.Fatalf("Latency75Pct unexpected %v", rs2.Latency75Pct)
	}
	if rs2.Latency90Pct != 616*time.Millisecond+100*time.Microsecond {
		t.Fatalf("Latency90Pct unexpected got %v", rs2.Latency90Pct)
	}
	if rs2.Latency99Pct != time.Second+390*time.Millisecond {
		t.Fatalf("Latency99Pct unexpected got %v", rs2.Latency99Pct)
	}
	if rs2.TotalRequests != 148205 {
		t.Fatalf("TotalRequests expected 148205, got %d", rs2.TotalRequests)
	}
	if rs2.TotalTook != 10*time.Second+30*time.Millisecond {
		t.Fatalf("TotalRequests expected 30s, got %v", rs2.TotalTook)
	}
	if rs2.TotalReadDataBytes != 5690000000 {
		t.Fatalf("TotalReadDataBytes expected 5690000000, got %d", rs2.TotalReadDataBytes)
	}
	if rs2.RequestsPerSec != 14775.76 {
		t.Fatalf("RequestsPerSec expected 14775.76, got %f", rs2.RequestsPerSec)
	}
	if rs2.TransferPerSecBytes != 580570000 {
		t.Fatalf("TransferPerSecBytes expected 580570000, got %d", rs2.TransferPerSecBytes)
	}
}
