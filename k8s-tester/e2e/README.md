# How to run tests and suites using Ginkgo

The main goal is to add ginkgo testing without inhibiting the function of the main binary tester.

In this scenario, we setup the `cfg` and `ts` inside the of each Tester execution. Then we run `Apply()` and `Delete()` in the tests. Any addition context can be added here. 

#### Basic Test from inside k8s-tester/e2e
```bash
go test -timeout=0 -v -ginkgo.v ./e2e_test.go
```

#### Specifying specific tests
```bash
go test -timeout=0 -v -ginkgo.v  -ginkgo.focus="Falco*" ./e2e_test.go
```

#### Running falco test directly with ginkgo
```bash
#pwd k8s-tester/falco
ginkgo -v
```

To add tests we add a tests .go file into each directory. We'll walk through adding `falco`.

1. create `falco-tests.go` in `/falco`
2. Create the tests implementing the `cfg` and `ts` interface
   ```go
    package falco

    import (
        "fmt"
        "os"
        "path/filepath"

        "github.com/aws/aws-k8s-tester/client"
        "github.com/aws/aws-k8s-tester/utils/log"
        "github.com/onsi/ginkgo"
        . "github.com/onsi/gomega"
        "go.uber.org/zap"
    )

    var (
        kubeconfigPath string
    )

    var _ = ginkgo.Describe("[Falco]", func() {
        if home := os.Getenv("HOME"); home != "" {
            kubeconfigPath = filepath.Join(home, ".kube", "config")
        } else {
            kubeconfigPath = client.DefaultKubectlPath()
        }
        lg, logWriter, _, _ := log.NewWithStderrWriter(log.DefaultLogLevel, []string{"stderr"})
        _ = zap.ReplaceGlobals(lg)
        cli, _ := client.New(&client.Config{
            Logger:         lg,
            KubeconfigPath: kubeconfigPath,
        })
        cfg := NewDefault()
        cfg.LogWriter = logWriter
        cfg.Logger = lg
        cfg.Enable = true
        cfg.Client = cli
        ts := New(cfg)
    })
   ```
3. Add the import to `e2e/e2e_test.go`


The test will now be added when we run the suite.