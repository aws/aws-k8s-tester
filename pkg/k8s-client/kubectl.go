package k8sclient

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Apply raw YAML data using kubectl
func (eks *eks) Apply(data string) error {
	return eks.executeKubectlWithTempFiles(data, "apply")
}

// Delete raw YAML data using kubectl
func (eks *eks) Delete(data string) error {
	return eks.executeKubectlWithTempFiles(data, "delete")
}

func (eks *eks) executeKubectlWithTempFiles(data string, verb string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 1. Create temp file
	file, err := ioutil.TempFile(".", "tmp-kubernetes-resource")
	if err != nil {
		return fmt.Errorf("while creating temporary file %s, %v", file.Name(), err)
	}
	defer os.Remove(file.Name())

	// 2. Write to temp file
	_, err = file.WriteString(data)
	if err != nil {
		return fmt.Errorf("while writing to file %s, %v", file.Name(), err)
	}

	// 3. Apply temp file
	output, err := exec.New().CommandContext(ctx, eks.cfg.KubectlPath, []string{
		fmt.Sprintf("--kubeconfig=%s", eks.cfg.KubeConfigPath), verb, "-f", file.Name(),
	}...).CombinedOutput()
	zap.S().Infof("%s", output)
	if err != nil {
		return fmt.Errorf("while applying file %s, with output %s, and error %v", file.Name(), output, err)
	}
	return nil
}
