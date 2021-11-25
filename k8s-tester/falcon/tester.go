// Package falcon install Falcon Operator
package falcon

import (
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`
}

func (cfg *Config) ValidateAndSetDefaults() error {

	return nil
}

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       false,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {

	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}
	return fmt.Errorf("NOT IMPLEMENTED")
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	return fmt.Errorf("NOT IMPLEMENTED")
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources, should we continue?", action)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}
