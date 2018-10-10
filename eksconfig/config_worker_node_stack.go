package eksconfig

import "fmt"

func genCFStackNodeGroup(clusterName string) string {
	return fmt.Sprintf("%s-NODE-GROUP-STACK", clusterName)
}
