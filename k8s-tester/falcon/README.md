# CrowdStrike Falcon Tester

## What tester does

 - Tester installs Falcon Operator on the cluster
 - tester deploys FalconContainer Custom Resource to the cluster
    - The operator then picks up the resource and installs CrowdStrike Falcon Container Workload Protection to the cluster
 - Tester verifies the Falcon Container is installed properly

## Exemplary usage:

 - Create new EKS cluster and obtain kubeconfig

 - Establish new API credentials with CrowdStrike Falcon plaform at https://falcon.crowdstrike.com/support/api-clients-and-keys; minimal required permissions are:
     Falcon Images Download: Read
     Sensor Download: Read

 - It is recommended to provide the API credentials to the tester by the means of environment variables
   ```
   export FALCON_CLIENT_ID="ASFD"
   export FALCON_CLIENT_SECRET="ASFD"
   ```

 - Apply the tester
   ```
   go run ./cmd/k8s-tester-falcon apply \
       --kubectl-path="$(which kubectl)" \
       --kubeconfig-path="$HOME/.kube/config"
   ```
 - Delete the tester
   ```
   go run ./cmd/k8s-tester-falcon delete \
       --kubectl-path="$(which kubectl)" \
       --kubeconfig-path="$HOME/.kube/config"
   ```

## Additional Resources
 - If you have read this far, you may be interested in [Debugging hints](https://github.com/CrowdStrike/falcon-operator/tree/main/docs/container#troubleshooting)
 - Learn more about [Falcon Operator](https://github.com/CrowdStrike/falcon-operator), [FalconContainer Resource](https://github.com/CrowdStrike/falcon-operator/tree/main/docs/container), [Falcon Container product page](https://www.crowdstrike.com/products/cloud-security/falcon-cloud-workload-protection/container-security/)
