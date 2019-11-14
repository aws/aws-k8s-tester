package cfn

import (
	"context"
	"fmt"
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
go test -v -run TestNewTemplate
RUN_AWS_TESTS=1 go test -v -run TestNewTemplate
*/
func TestNewTemplate(t *testing.T) {
	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	tmpl, err := NewTemplate(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var body []byte
	body, err = tmpl.JSON()
	if err != nil {
		t.Fatal(err)
	}
	templateBodyYAML := string(body)
	fmt.Println(templateBodyYAML)
	// t.Skip()

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
		TemplateBody: aws.String(templateBodyYAML),
	})
	if err != nil {
		t.Fatal(err)
	}
	stackID := aws.StringValue(cout.StackId)
	fmt.Println("Created stack:", stackID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awsapi_cfn.Wait(ctx, lg, svc, stackID, svccfn.ResourceStatusCreateComplete)
	for st := range ch {
		fmt.Println(st.Stack.GoString(), st.Error)
	}
	cancel()

	time.Sleep(10 * time.Second)

	if _, err = svc.DeleteStack(&svccfn.DeleteStackInput{StackName: aws.String(stackID)}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	ch = awsapi_cfn.Wait(ctx, lg, svc, stackID, svccfn.ResourceStatusDeleteComplete)
	for st := range ch {
		fmt.Println(st.Stack.GoString(), st.Error)
	}
	cancel()
}
