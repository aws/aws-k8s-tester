package tester

import (
	"fmt"
	"log"
	"math/rand"
	"os"
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

//TODO: catch SIGTERM and do clean up
func (t *Tester) Start() error {
	err := t.init.Run()
	if err != nil {
		return err
	}

	err = t.build.Run()
	if err != nil {
		return err
	}

	err = t.up.Run()
	if err != nil {
		log.Printf("Up failed: %s", err)
		tErr := t.tearDown.Run()
		if tErr != nil {
			log.Printf("failed to tear down cluster %s", tErr)
		}
		return err
	}

	err = t.install.Run()
	if err != nil {
		tErr := t.tearDown.Run()
		if tErr != nil {
			log.Printf("failed to tear down cluster: %v", tErr)
		}
		return err
	}

	err = t.test.Run()
	if uninstallErr := t.uninstall.Run(); uninstallErr != nil {
		log.Printf("Failed to run install step: %s", uninstallErr)
	}
	if tearDownErr := t.tearDown.Run(); tearDownErr != nil {
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
