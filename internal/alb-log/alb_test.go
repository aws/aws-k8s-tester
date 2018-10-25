package alblog

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestALB(t *testing.T) {
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
	ll := `http 2018-09-30T07:55:04.473154Z app/99b85e8b-default-ingressfo-af34/12e524a93c4f4e11 205.251.233.48:2432 192.168.70.117:31188 0.002 0.001 0.000 200 200 205 41235 "GET http://99b85e8b-default-ingressfo-af34-1499232465.us-west-2.elb.amazonaws.com:80/ingress-test HTTP/1.1" "Go-http-client/1.1" - - arn:aws:elasticloadbalancing:us-west-2:607362164682:targetgroup/99b85e8b-cc75a0156f481c09183/9e2765f9f73806a9 "Root=1-5bb08158-b6f9ff021c04f3781939905f" "-" "-" 1 2018-09-30T07:55:04.469000Z "forward" "-"`
	ls, err := splitLog(ll)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 24 {
		t.Fatalf("fields expected 24, got %d", len(ls))
	}

	logs, err := Parse("alb.log")
	if err != nil {
		t.Fatal(err)
	}
	requestProcessingTimes := make(map[float64]int)
	targetProcessingTimes := make(map[float64]int)
	responseProcessingTimes := make(map[float64]int)
	for _, v := range logs {
		requestProcessingTimes[v.RequestProcessingTimeSeconds]++
		targetProcessingTimes[v.TargetProcessingTimeSeconds]++
		responseProcessingTimes[v.ResponseProcessingTimeSeconds]++
	}
	fmt.Println("requestProcessingTimes:", requestProcessingTimes)
	fmt.Println("targetProcessingTimes:", targetProcessingTimes)
	fmt.Println("responseProcessingTimes:", responseProcessingTimes)

	f, err := ioutil.TempFile(os.TempDir(), "elblog")
	if err != nil {
		t.Fatal(err)
	}
	output := f.Name()
	f.Close()
	os.RemoveAll(output)
	defer os.RemoveAll(output)
	if err = ConvertToCSV(output, "alb.log"); err != nil {
		t.Fatal(err)
	}
}
