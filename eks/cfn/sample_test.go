package cfn

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	awsapi_cfn "github.com/aws/aws-k8s-tester/pkg/awsapi/cloudformation"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	svccfn "github.com/aws/aws-sdk-go/service/cloudformation"
)

/*
go test -v -timeout 2h -run TestSample
RUN_AWS_TESTS=1 go test -v -timeout 2h -run TestSample
*/
func TestSample(t *testing.T) {
	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	body, err := ioutil.ReadFile("sample.yaml")
	if err != nil {
		t.Fatal(err)
	}
	templateBodyYAML := string(body)
	fmt.Println(string(templateBodyYAML))

	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	lg, _ := logutil.GetDefaultZapLogger()
	awsCfg := &awsapi.Config{
		Logger:        lg,
		DebugAPICalls: false,
		Region:        "us-west-2",
	}
	ss, stsOutput, _, err := awsapi.New(awsCfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Running with:", *stsOutput.Account)

	svc := svccfn.New(ss)
	var cout *svccfn.CreateStackOutput
	cout, err = svc.CreateStack(&svccfn.CreateStackInput{
		StackName:    aws.String(cfg.ClusterName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
		OnFailure:    aws.String("DELETE"),
		TemplateBody: aws.String(templateBodyYAML),
		Tags: awsapi_cfn.NewCloudFormationTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": cfg.ClusterName,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	stackID := aws.StringValue(cout.StackId)
	fmt.Println("Created stack:", stackID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awsapi_cfn.Wait(ctx, lg, svc, stackID, svccfn.ResourceStatusCreateComplete)
	for st := range ch {
		if st.Stack != nil {
			fmt.Println(st.Stack.GoString(), st.Error)
		} else {
			fmt.Println(st.Stack, st.Error)
		}
	}
	cancel()

	time.Sleep(10 * time.Second)

	if _, err = svc.DeleteStack(&svccfn.DeleteStackInput{StackName: aws.String(stackID)}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	ch = awsapi_cfn.Wait(ctx, lg, svc, stackID, svccfn.ResourceStatusDeleteComplete)
	for st := range ch {
		if st.Stack != nil {
			fmt.Println(st.Stack.GoString(), st.Error)
		} else {
			fmt.Println(st.Stack, st.Error)
		}
	}
	cancel()
}
