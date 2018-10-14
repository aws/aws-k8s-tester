package ingress

import (
	"fmt"
	"strings"

	gyaml "github.com/ghodss/yaml"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const configMapNginxTempl = `---
apiVersion: v1
kind: ConfigMap

metadata:
  name: nginx-default-conf

data:
  default.conf: ""

---
apiVersion: v1
kind: ConfigMap

metadata:
  name: nginx-index-html

data:
  index.html: ""

`

const (
	nginxDefaultConf = `server {
  listen 80 default_server;
  listen [::]:80 default_server;
  server_name _;
  root /usr/share/nginx/html;
  location / {
  }
}
`
)

// CreateConfigMapNginx creates an Nginx config map to apply.
func CreateConfigMapNginx(responseSize int) (string, error) {
	cm1 := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-default-conf",
		},
		Data: map[string]string{
			"default.conf": nginxDefaultConf,
		},
	}

	cm2 := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-index-html",
		},
		Data: map[string]string{
			"index.html": strings.Repeat("0", responseSize),
		},
	}

	d1, err := gyaml.Marshal(cm1)
	if err != nil {
		return "", err
	}
	d2, err := gyaml.Marshal(cm2)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`---
%s


---
%s


`, string(d1), string(d2)), nil
}
