package plugins

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

// headerBash is the bash script header.
const headerBash = `#!/usr/bin/env bash`

// READY is appended on init script complete.
const READY = "AWSTESTER_EC2_PLUGIN_READY"

type script struct {
	key  string
	data string
}

type scripts []script

func (ss scripts) Len() int           { return len(ss) }
func (ss scripts) Swap(i, j int)      { ss[i], ss[j] = ss[j], ss[i] }
func (ss scripts) Less(i, j int) bool { return keyPriorities[ss[i].key] < keyPriorities[ss[j].key] }

var keyPriorities = map[string]int{ // in the order of:
	"update-ubuntu":  1,
	"mount-aws-cred": 2,
	"install-go":     3,
	"install-csi":    4,
	"install-wrk":    5,
	"install-alb":    6,
}

func convertToScript(userName, plugin string) (script, error) {
	switch {
	case plugin == "update-ubuntu":
		return script{key: "update-ubuntu", data: updateUbuntu}, nil

	case strings.HasPrefix(plugin, "mount-aws-cred-"):
		env := strings.Replace(plugin, "mount-aws-cred-", "", -1)
		if os.Getenv(env) == "" {
			return script{}, fmt.Errorf("%q is not defined", env)
		}
		d, derr := ioutil.ReadFile(os.Getenv(env))
		if derr != nil {
			return script{}, derr
		}
		return script{
			key: "mount-aws-cred",
			data: fmt.Sprintf(`
mkdir -p /home/%s/.aws/

cat << EOT > /home/%s/.aws/credentials
%s
EOT`, userName, userName, string(d)),
		}, nil

	case plugin == "install-go1.11.1":
		s, err := createInstallGo(goInfo{
			UserName:  userName,
			GoVersion: "1.11.1",
		})
		if err != nil {
			return script{}, err
		}
		return script{
			key:  "install-go",
			data: s,
		}, nil

	case plugin == "install-wrk":
		return script{
			key:  plugin,
			data: wrk,
		}, nil

	case strings.HasPrefix(plugin, "install-csi-"):
		gitBranch := strings.Replace(plugin, "install-csi-", "", -1)
		_, perr := strconv.ParseInt(gitBranch, 10, 64)
		isPR := perr == nil
		s, err := createInstallGit(gitInfo{
			GitName:       "kubernetes-sigs",
			GitRepoName:   "aws-ebs-csi-driver",
			IsPR:          isPR,
			GitBranch:     gitBranch,
			InstallScript: `go install -v ./cmd/aws-ebs-csi-driver`,
		})
		if err != nil {
			return script{}, err
		}
		return script{key: "install-csi", data: s}, nil

	case strings.HasPrefix(plugin, "install-alb-"):
		gitBranch := strings.Replace(plugin, "install-alb-", "", -1)
		_, perr := strconv.ParseInt(gitBranch, 10, 64)
		isPR := perr == nil
		s, err := createInstallGit(gitInfo{
			GitName:     "kubernetes-sigs",
			GitRepoName: "aws-alb-ingress-controller",
			IsPR:        isPR,
			GitBranch:   gitBranch,
			InstallScript: `GO111MODULE=on go mod vendor -v
			make server
			`,
		})
		if err != nil {
			return script{}, err
		}
		return script{key: "install-alb", data: s}, nil
	}

	return script{}, fmt.Errorf("unknown plugin %q", plugin)
}

// Get returns the plugin.
func Get(userName string, plugins []string) (data string, err error) {
	sts := make([]script, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin == "update-ubuntu" {
			if userName != "ubuntu" {
				return "", fmt.Errorf("'update-ubuntu' requires 'ubuntu' user name, got %q", userName)
			}
		}

		script, err := convertToScript(userName, plugin)
		if err != nil {
			return "", err
		}
		sts = append(sts, script)
	}
	sort.Sort(scripts(sts))

	data = headerBash
	for _, s := range sts {
		data += s.data
	}
	data += fmt.Sprintf("\n\necho %s\n\n", READY)
	return data, nil
}

const updateUbuntu = `

################################## update Ubuntu

export HOME=/home/ubuntu
export GOPATH=/home/ubuntu/go

apt-get -y update \
  && apt-get -y install \
  build-essential \
  gcc \
  jq \
  file \
  apt-utils \
  pkg-config \
  software-properties-common \
  apt-transport-https \
  ca-certificates \
  libssl-dev \
  gnupg2 \
  sudo \
  bash \
  curl \
  wget \
  tar \
  git \
  mercurial \
  openssh-client \
  rsync \
  unzip \
  wget \
  xz-utils \
  zip \
  zlib1g-dev \
  lsb-release \
  python3 \
  python3-pip \
  python3-setuptools \
  && apt-get clean \
  && pip3 install awscli --no-cache-dir --upgrade \
  && which aws && aws --version \
  && apt-get -y install \
  python \
  python-dev \
  python-openssl \
  python-pip \
  && pip install --upgrade pip setuptools wheel

##################################

`

func createInstallGo(g goInfo) (string, error) {
	tpl := template.Must(template.New("installGoTemplate").Parse(installGoTemplate))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, g); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type goInfo struct {
	UserName  string
	GoVersion string
}

const installGoTemplate = `

################################## install Go

export HOME=/home/{{ .UserName }}
export GOPATH=/home/{{ .UserName }}/go

GO_VERSION={{ .GoVersion }}
GOOGLE_URL=https://storage.googleapis.com/golang
DOWNLOAD_URL=${GOOGLE_URL}

sudo curl -s ${DOWNLOAD_URL}/go$GO_VERSION.linux-amd64.tar.gz | sudo tar -v -C /usr/local/ -xz

mkdir -p ${GOPATH}/bin/
mkdir -p ${GOPATH}/src/github.com/kubernetes-sigs
mkdir -p ${GOPATH}/src/k8s.io
mkdir -p ${GOPATH}/src/sigs.k8s.io

if grep -q GOPATH "${HOME}/.bashrc"; then
  echo "bashrc already has GOPATH";
else
  echo "adding GOPATH to bashrc";
  echo "export GOPATH=${HOME}/go" >> ${HOME}/.bashrc;
  PATH_VAR=$PATH":/usr/local/go/bin:${HOME}/go/bin";
  echo "export PATH=$(echo $PATH_VAR)" >> ${HOME}/.bashrc;
  source ${HOME}/.bashrc;
fi

source ${HOME}/.bashrc
export PATH=$PATH:/usr/local/go/bin:${HOME}/go/bin

sudo echo PATH=${PATH} > /etc/environment
sudo echo GOPATH=/home/{{ .UserName }}/go >> /etc/environment

go version

##################################

`

const wrk = `

################################## install wrk

cd ${HOME}

RETRIES=10
DELAY=10
COUNT=1
while [[ ${COUNT} -lt ${RETRIES} ]]; do
  rm -rf ./wrk
  git clone https://github.com/wg/wrk.git
  if [[ $? -eq 0 ]]; then
    RETRIES=0
    echo "Successully git cloned!"
    break
  fi
  let COUNT=${COUNT}+1
  sleep ${DELAY}
done

pushd wrk \
  && make all \
  && sudo cp ./wrk /usr/local/bin/wrk \
  && popd \
  && rm -rf ./wrk \
  && wrk --version || true && which wrk

##################################

`

func createInstallGit(g gitInfo) (string, error) {
	if g.IsPR {
		_, serr := strconv.ParseInt(g.GitBranch, 10, 64)
		if serr != nil {
			return "", fmt.Errorf("expected PR number, got %q (%v)", g.GitBranch, serr)
		}
	}
	tpl := template.Must(template.New("installGitTemplate").Parse(installGitTemplate))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, g); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type gitInfo struct {
	GitName       string
	GitRepoName   string
	IsPR          bool
	GitBranch     string
	InstallScript string
}

const installGitTemplate = `

################################## install {{ .GitRepoName }} via git

mkdir -p ${GOPATH}/src/github.com/{{ .GitName }}/
cd ${GOPATH}/src/github.com/{{ .GitName }}/

RETRIES=10
DELAY=10
COUNT=1
while [[ ${COUNT} -lt ${RETRIES} ]]; do
  rm -rf ./{{ .GitRepoName }}
  git clone https://github.com/{{ .GitName }}/{{ .GitRepoName }}.git
  if [[ $? -eq 0 ]]; then
    RETRIES=0
    echo "Successully git cloned!"
    break
  fi
  let COUNT=${COUNT}+1
  sleep ${DELAY}
done

cd ${GOPATH}/src/github.com/{{ .GitName }}/{{ .GitRepoName }}

{{ if .IsPR }}echo 'git fetching:' pull/{{ .GitBranch }}/head 'to test branch'
git fetch origin pull/{{ .GitBranch }}/head:test
git checkout test
{{ else }}
git checkout origin/{{ .GitBranch }}
git checkout -B {{ .GitBranch }}
{{ end }}

{{ .InstallScript }}

git remote -v
git branch
git log --pretty=oneline -5

##################################

`
