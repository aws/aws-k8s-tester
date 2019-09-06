package tester

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/tester/pkg"
	yaml "gopkg.in/yaml.v2"
)

var rnd *rand.Rand

func init() {
	rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
}

type Tester struct {
	init      pkg.Step
	build     pkg.Step
	up        pkg.Step
	install   pkg.Step
	test      pkg.Step
	uninstall pkg.Step
	tearDown  pkg.Step
}

func NewTester(configPath string) *Tester {
	if configPath == "" {
		configPath = readTestConfigPath()
	}
	testConfig, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	var config *pkg.TestConfig
	err = yaml.NewDecoder(testConfig).Decode(&config)
	if err != nil {
		panic(err)
	}

	testId := fmt.Sprintf("%d", rnd.Intn(10000))
	clusterCreator, err := pkg.NewClusterCreator(config, "/tmp/tester-e2e-test", testId)
	if err != nil {
		panic(err)
	}

	return &Tester{
		init:      createStepOrPanic(clusterCreator.Init),
		build:     scriptStep(config.BuildScript, testId),
		up:        createStepOrPanic(clusterCreator.Up),
		install:   scriptStep(config.InstallScript, testId),
		test:      scriptStep(config.TestScript, testId),
		uninstall: scriptStep(config.UninstallScript, testId),
		tearDown:  createStepOrPanic(clusterCreator.TearDown),
	}
}

func (t *Tester) Start(ctx context.Context) error {
	err := t.init.Run(ctx)
	if err != nil {
		return err
	}

	err = t.build.Run(ctx)
	if err != nil {
		return err
	}

	err = t.up.Run(ctx)
	if err != nil {
		log.Printf("Up failed: %s", err)
		tErr := t.tearDown.Run(context.Background())
		if tErr != nil {
			log.Printf("failed to tear down cluster %s", tErr)
		}
		return err
	}

	err = t.install.Run(ctx)
	if err != nil {
		tErr := t.tearDown.Run(context.Background())
		if tErr != nil {
			log.Printf("failed to tear down cluster: %v", tErr)
		}
		return err
	}

	err = t.test.Run(ctx)
	if uninstallErr := t.uninstall.Run(context.Background()); uninstallErr != nil {
		log.Printf("Failed to run install step: %s", uninstallErr)
	}
	if tearDownErr := t.tearDown.Run(context.Background()); tearDownErr != nil {
		log.Printf("Failed to run tear down step: %s", tearDownErr)
	}

	return err
}

func readTestConfigPath() string {
	path := os.Getenv("TESTCONFIG")
	if len(path) == 0 {
		return "test-config.yaml"
	}

	return path
}

// Helper function for running the test
func Start() {
	test := NewTester("")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// test terminates after SIGTERM or after 90 minutes timeout
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
		timeout := 100 * time.Minute
		timer := time.NewTimer(timeout)

		select {
		case sig := <-sigs:
			log.Printf("Received signal %s Cancel the workflow", sig)
			cancel()
		case <-timer.C:
			log.Printf("Test times out after %s Cancel the workflow", timeout)
			cancel()
		}
	}()

	err := test.Start(ctx)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func createStepOrPanic(f func() (pkg.Step, error)) pkg.Step {
	step, err := f()
	if err != nil {
		panic(err)
	}
	return step
}

func scriptStep(script string, testId string) pkg.Step {
	return &pkg.TestStep{Script: script, TestId: testId}
}
