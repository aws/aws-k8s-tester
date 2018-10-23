package plugins

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// headerBash is the bash script header.
const headerBash = `#!/usr/bin/env bash`

// READY is appended on init script complete.
const READY = "PLUGIN_READY"

var pluginToTempl = map[string]string{
	"update-ubuntu":           update_ubuntu,
	"mount-aws-cred-":         aws_cred,
	"install-go1.11.1-ubuntu": go_1_11_1_ubuntu,
	"install-wrk":             wrk,
	"install-csi-master":      install_csi_master,
	"install-csi-":            install_csi_pr,
}

// Get returns the plugin.
func Get(ps ...string) (ss string, err error) {
	ss = headerBash
	pfxToKey := make(map[string]string)

	// AWS_SHARED_CREDENTIALS_FILE
	for _, p := range ps {
		switch {
		case p == "update-ubuntu":
			pfxToKey[p] = p

		case p == "mount-aws-cred-":
			return "", errors.New("unknown AWS credential path")

		case strings.HasPrefix(p, "mount-aws-cred-"):
			pfxToKey["mount-aws-cred-"] = p

		case p == "install-go1.11.1-ubuntu":
			pfxToKey[p] = p

		case p == "install-wrk":
			pfxToKey[p] = p

		case p == "install-csi-master":
			pfxToKey[p] = p

		case p == "install-csi-":
			return "", errors.New("unknown CSI Pull Number")

		case strings.HasPrefix(p, "install-csi-"):
			pfxToKey["install-csi-"] = p

		default:
			return "", fmt.Errorf("plugin %q not found", p)
		}
	}

	// to ensure the ordering
	csiMasterFound := false
	userName := ""
	for _, pfx := range []string{
		"update-ubuntu",
		"mount-aws-cred-",
		"install-go1.11.1-ubuntu",
		"install-wrk",
		"install-csi-master",
		"install-csi-",
	} {
		key, ok := pfxToKey[pfx]
		if !ok {
			continue
		}
		if pfx == "install-csi-master" {
			csiMasterFound = true
		}

		txt := pluginToTempl[pfx]
		switch pfx {
		case "update-ubuntu":
			userName = "ubuntu"

		case "mount-aws-cred-":
			cred := strings.Replace(key, "mount-aws-cred-", "", -1)

			if os.Getenv(cred) == "" {
				return "", fmt.Errorf("%q is not defined", cred)
			}
			d, derr := ioutil.ReadFile(os.Getenv(cred))
			if derr != nil {
				return "", derr
			}
			txt = fmt.Sprintf(txt, userName, userName, string(d))

		case "install-csi-":
			if csiMasterFound {
				return "", errors.New("'install-csi-master' already specified")
			}

			pullNumber := strings.Replace(key, "install-csi-", "", -1)
			txt = fmt.Sprintf(txt, pullNumber, pullNumber)
		}

		ss += txt
	}

	ss += "\n\necho PLUGIN_READY\n\n"
	return ss, nil
}
