package plugins

const install_csi_master = `
mkdir -p ${GOPATH}/src/github.com/kubernetes-sigs/
cd ${GOPATH}/src/github.com/kubernetes-sigs/

RETRIES=10
DELAY=10
COUNT=1
while [[ ${COUNT} -lt $RETRIES ]]; do
  rm -rf ./aws-ebs-csi-driver
  git clone https://github.com/kubernetes-sigs/aws-ebs-csi-driver.git
  if [[ $? -eq 0 ]]; then
    RETRIES=0
    echo "Successully git cloned!"
    break
  fi
  let COUNT=${COUNT}+1
  sleep ${DELAY}
done

cd ${GOPATH}/src/github.com/kubernetes-sigs/aws-ebs-csi-driver

go install -v ./cmd/aws-ebs-csi-driver

git remote -v
git branch
git log --pretty=oneline -10

`

const install_csi_pr = `
mkdir -p ${GOPATH}/src/github.com/kubernetes-sigs/
cd ${GOPATH}/src/github.com/kubernetes-sigs/

RETRIES=10
DELAY=10
COUNT=1
while [[ ${COUNT} -lt $RETRIES ]]; do
  rm -rf ./aws-ebs-csi-driver
  git clone https://github.com/kubernetes-sigs/aws-ebs-csi-driver.git
  if [[ $? -eq 0 ]]; then
    RETRIES=0
    echo "Successully git cloned!"
    break
  fi
  let COUNT=${COUNT}+1
  sleep ${DELAY}
done

cd ${GOPATH}/src/github.com/kubernetes-sigs/aws-ebs-csi-driver

echo 'git fetching:' pull/%s/head 'to test branch'
git fetch origin pull/%s/head:test
git checkout test

go install -v ./cmd/aws-ebs-csi-driver

git remote -v
git branch
git log --pretty=oneline -10

`
