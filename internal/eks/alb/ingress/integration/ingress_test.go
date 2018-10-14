package ingress_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/aws/awstester/internal/eks/alb/ingress/client"
	"github.com/aws/awstester/internal/eks/alb/ingress/server"

	"go.uber.org/zap"
)

func TestIngress(t *testing.T) {
	routesN := 3

	// start server
	mux, err := server.NewMux(context.Background(), zap.NewExample(), routesN, 10)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// send loads from client to server
	cli, err := client.New(zap.NewExample(), ts.URL, routesN, 10, 100)
	if err != nil {
		t.Fatal(err)
	}

	o := cli.Run()
	fmt.Printf("%+v\n", o)

	if len(o.Errors) != 0 {
		t.Fatalf("unexpected %d errors", o.Errors)
	}
}
