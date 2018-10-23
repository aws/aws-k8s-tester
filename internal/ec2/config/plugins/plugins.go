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
	"update-ubuntu":           updateUbuntu,
	"mount-aws-cred":          awsCred,
	"install-go1.11.1-ubuntu": go1111Ubuntu,
	"install-wrk":             wrk,
	"install-csi-master":      csiCheckoutMaster,
	"install-csi-":            csiCheckoutPR,
}

// Get returns the plugin.
func Get(ps ...string) (ss string, err error) {
	ss = headerBash
	pfxToKey := make(map[string]string)

	for _, p := range ps {
		switch {
		case p == "update-ubuntu":
			pfxToKey[p] = p
		case p == "mount-aws-cred":
			pfxToKey[p] = p
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
	for _, key := range []string{
		"update-ubuntu",
		"mount-aws-cred",
		"install-go1.11.1-ubuntu",
		"install-wrk",
		"install-csi-master",
		"install-csi-",
	} {
		if v, ok := pfxToKey[key]; ok {
			if key == "install-csi-master" {
				csiMasterFound = true
			}

			txt := pluginToTempl[key]
			switch key {
			case "update-ubuntu":
				userName = "ubuntu"

			case "mount-aws-cred":
				if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") == "" {
					return "", errors.New("AWS_SHARED_CREDENTIALS_FILE is not defined")
				}
				d, derr := ioutil.ReadFile(os.Getenv("AWS_SHARED_CREDENTIALS_FILE"))
				if derr != nil {
					return "", derr
				}
				txt = fmt.Sprintf(txt, userName, userName, string(d))

			case "install-csi-":
				if csiMasterFound {
					return "", errors.New("'install-csi-master' already specified")
				}

				pullNumber := strings.Split(v, "-")[2]
				txt = fmt.Sprintf(txt, pullNumber, pullNumber)
			}

			ss += txt
		}
	}

	ss += "\n\necho PLUGIN_READY\n\n"
	return ss, nil
}
