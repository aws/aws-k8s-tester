package k8sclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	deprecate "github.com/aws/aws-k8s-tester/pkg/k8s-client/eks-deprecate"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
)

const scriptHeader = `#!/bin/bash
set -xeu

`

func (e *eks) Deprecate() (err error) {
	rbPath := filepath.Join(e.cfg.Dir, "rollback.sh")
	var rbF *os.File
	rbF, err = createBashScript(rbPath)
	if err != nil {
		return err
	}
	defer func() {
		rbF.Close()
		fileutil.EnsureExecutable(rbPath)
	}()
	upPath := filepath.Join(e.cfg.Dir, "upgrade.sh")
	var upF *os.File
	upF, err = createBashScript(upPath)
	if err != nil {
		return err
	}
	defer func() {
		upF.Close()
		fileutil.EnsureExecutable(upPath)
	}()

	getCmd := []string{e.cfg.KubectlPath, "--kubeconfig=" + e.cfg.KubeConfigPath, "get", "all"}
	_, err = rbF.Write([]byte(strings.Join(getCmd, " ") + "\n\n"))
	if err != nil {
		return err
	}
	_, err = upF.Write([]byte(strings.Join(getCmd, " ") + "\n\n"))
	if err != nil {
		return err
	}

	ver, err := e.fetchServerVersion()
	if err != nil {
		return err
	}
	verTxt, err := json.MarshalIndent(ver, "", "    ")
	if err != nil {
		return err
	}

	cur := ver.VersionValue
	new := cur + 0.01

	apis, err := deprecate.APIs(new)
	if err != nil {
		e.cfg.Logger.Warn("version not supported", zap.Error(err))
		return nil
	}
	deprecates := make([]string, 0, len(apis))
	for k := range apis {
		deprecates = append(deprecates, fmt.Sprintf("%s.%s", k.APIVersion, k.Kind))
	}
	sort.Strings(deprecates)

	ns, err := e.listNamespaces(20, time.Second)
	if err != nil {
		return err
	}
	namespaces := make([]string, 0, len(ns))
	for _, nv := range ns {
		namespaces = append(namespaces, nv.GetName())
	}
	sort.Strings(namespaces)

	e.cfg.Logger.Info("😎 🙏 🙏  checking deprecated APIs",
		zap.Bool("enable-prompt", e.cfg.EnablePrompt),
		zap.Int64("list-batch", e.cfg.ListBatch),
		zap.Duration("list-interval", e.cfg.ListInterval),
		zap.String("rollback-script", rbPath),
		zap.String("upgrade-script", upPath),
		zap.String("version-current", fmt.Sprintf("%.2f", cur)),
		zap.String("version-target", fmt.Sprintf("%.2f", new)),
		zap.Strings("namespaces", namespaces),
		zap.Strings("deprecates", deprecates),
	)
	fmt.Printf("\n%s\n\n", string(verTxt))

	if e.cfg.EnablePrompt {
		prompt := promptui.Select{
			Label: "Ready to list all resources to find deprecated APIs, should we continue?",
			Items: []string{
				"No, stop it!",
				"Yes, let's find them all!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			return err
		}
		if idx != 1 {
			e.cfg.Logger.Info("returning", zap.Int("index", idx), zap.String("answer", answer))
			return nil
		}
	}

	// TODO: this runs highly redundant queries... optimize...
	//  1. find all resources with the "Namespace" and "Kind"
	//  2. decide whether to deprecate based on "kubectl get -o=yaml"
	fmt.Printf("\n\n************************\n")
	e.cfg.Logger.Info("listing all resources to find deprecated APIs")
	for from, to := range apis {
		fmt.Printf("\n\n************************\n")
		switch {
		case from.APIVersion == "apps/v1beta1" && from.Kind == "Deployment",
			from.APIVersion == "apps/v1beta2" && from.Kind == "Deployment",
			from.APIVersion == "extensions/v1beta1" && from.Kind == "Deployment":
			for _, namespace := range namespaces {
				e.cfg.Logger.Info("checking",
					zap.String("from-api-version", from.APIVersion),
					zap.String("from-kind", from.Kind),
					zap.String("to-api-version", to.APIVersion),
					zap.String("to-kind", to.Kind),
					zap.String("namespace", namespace),
				)

				rs1, err := e.ListAppsV1beta1Deployments(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs2, err := e.ListAppsV1beta2Deployments(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs3, err := e.ListAppsV1Deployments(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs4, err := e.ListExtensionsV1beta1Deployments(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}

				if len(rs1) == 0 && len(rs2) == 0 && len(rs3) == 0 && len(rs4) == 0 {
					e.cfg.Logger.Info("😁 😁 😁  skipping; no resource found",
						zap.String("from-api-version", from.APIVersion),
						zap.String("from-kind", from.Kind),
						zap.String("namespace", namespace),
					)
					time.Sleep(e.cfg.ListInterval)
					continue
				}
				resources := make(map[string]struct{})
				for _, v := range rs1 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs2 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs3 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs4 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				allNames := make([]string, 0, len(resources))
				for k := range resources {
					allNames = append(allNames, k)
				}
				sort.Strings(allNames)
				e.cfg.Logger.Info("checking all names", zap.String("namespace", namespace), zap.Strings("names", allNames))
				for _, name := range allNames {
					time.Sleep(100 * time.Millisecond)

					orig, origBody, err := e.GetObject(namespace, from.Kind, name)
					if err != nil {
						return err
					}

					if orig.APIVersion == "" || orig.APIVersion == to.APIVersion {
						e.cfg.Logger.Warn("😁  skipping latest API",
							zap.String("namespace", namespace),
							zap.String("name", name),
							zap.String("current-api-version", orig.APIVersion),
							zap.String("expected-api-version", to.APIVersion),
						)
						continue
					}

					e.cfg.Logger.Warn("😱 😱 😱  found deprecated API!",
						zap.String("namespace", namespace),
						zap.String("name", name),
						zap.String("current-api-version", orig.APIVersion),
						zap.String("expected-api-version", to.APIVersion),
					)

					if err = e.saveKubectlGet(namespace, orig.Kind, name, rbF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlGet(namespace, orig.Kind, name, upF, "\n"); err != nil {
						return err
					}
					patchBody := strings.Replace(
						string(origBody),
						"apiVersion: "+orig.APIVersion+"\n",
						"apiVersion: "+to.APIVersion+"\n",
						1,
					)

					origYAMLPath, err := e.saveYAML(namespace, orig.APIVersion, orig.Kind, name, ".original.yaml", origBody)
					if err != nil {
						return err
					}
					patchYAMLPath, err := e.saveYAML(namespace, to.APIVersion, to.Kind, name, ".patch.yaml", []byte(patchBody))
					if err != nil {
						return err
					}

					if err = e.saveKubectlApply(origYAMLPath, rbF, "\n\n"); err != nil {
						return err
					}
					if err = e.saveKubectlConvert(namespace, from.Kind, to.APIVersion, name, upF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlApply(patchYAMLPath, upF, "\n\n"); err != nil {
						return err
					}
				}
			}

		case from.APIVersion == "apps/v1beta1" && from.Kind == "StatefulSet",
			from.APIVersion == "apps/v1beta2" && from.Kind == "StatefulSet":
			for _, namespace := range namespaces {
				e.cfg.Logger.Info("checking",
					zap.String("from-api-version", from.APIVersion),
					zap.String("from-kind", from.Kind),
					zap.String("to-api-version", to.APIVersion),
					zap.String("to-kind", to.Kind),
					zap.String("namespace", namespace),
				)

				rs1, err := e.ListAppsV1beta1StatefulSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs2, err := e.ListAppsV1beta2StatefulSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs3, err := e.ListAppsV1StatefulSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}

				if len(rs1) == 0 && len(rs2) == 0 && len(rs3) == 0 {
					e.cfg.Logger.Info("😁 😁 😁  skipping; no resource found",
						zap.String("from-api-version", from.APIVersion),
						zap.String("from-kind", from.Kind),
						zap.String("namespace", namespace),
					)
					time.Sleep(e.cfg.ListInterval)
					continue
				}
				resources := make(map[string]struct{})
				for _, v := range rs1 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs2 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs3 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				allNames := make([]string, 0, len(resources))
				for k := range resources {
					allNames = append(allNames, k)
				}
				sort.Strings(allNames)
				e.cfg.Logger.Info("checking all names", zap.String("namespace", namespace), zap.Strings("names", allNames))
				for _, name := range allNames {
					time.Sleep(100 * time.Millisecond)

					orig, origBody, err := e.GetObject(namespace, from.Kind, name)
					if err != nil {
						return err
					}

					if orig.APIVersion == "" || orig.APIVersion == to.APIVersion {
						e.cfg.Logger.Warn("😁  skipping latest API",
							zap.String("namespace", namespace),
							zap.String("name", name),
							zap.String("current-api-version", orig.APIVersion),
							zap.String("expected-api-version", to.APIVersion),
						)
						continue
					}

					e.cfg.Logger.Warn("😱 😱 😱  found deprecated API!",
						zap.String("namespace", namespace),
						zap.String("name", name),
						zap.String("current-api-version", orig.APIVersion),
						zap.String("expected-api-version", to.APIVersion),
					)

					if err = e.saveKubectlGet(namespace, orig.Kind, name, rbF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlGet(namespace, orig.Kind, name, upF, "\n"); err != nil {
						return err
					}
					patchBody := strings.Replace(
						string(origBody),
						"apiVersion: "+orig.APIVersion+"\n",
						"apiVersion: "+to.APIVersion+"\n",
						1,
					)

					origYAMLPath, err := e.saveYAML(namespace, orig.APIVersion, orig.Kind, name, ".original.yaml", origBody)
					if err != nil {
						return err
					}
					patchYAMLPath, err := e.saveYAML(namespace, to.APIVersion, to.Kind, name, ".patch.yaml", []byte(patchBody))
					if err != nil {
						return err
					}

					if err = e.saveKubectlApply(origYAMLPath, rbF, "\n\n"); err != nil {
						return err
					}
					if err = e.saveKubectlConvert(namespace, from.Kind, to.APIVersion, name, upF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlApply(patchYAMLPath, upF, "\n\n"); err != nil {
						return err
					}
				}
			}

		case from.APIVersion == "extensions/v1beta1" && from.Kind == "DaemonSet":
			for _, namespace := range namespaces {
				e.cfg.Logger.Info("checking",
					zap.String("from-api-version", from.APIVersion),
					zap.String("from-kind", from.Kind),
					zap.String("to-api-version", to.APIVersion),
					zap.String("to-kind", to.Kind),
					zap.String("namespace", namespace),
				)

				rs1, err := e.ListExtensionsV1beta1DaemonSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs2, err := e.ListAppsV1DaemonSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}

				if len(rs1) == 0 && len(rs2) == 0 {
					e.cfg.Logger.Info("😁 😁 😁  skipping; no resource found",
						zap.String("from-api-version", from.APIVersion),
						zap.String("from-kind", from.Kind),
						zap.String("namespace", namespace),
					)
					time.Sleep(e.cfg.ListInterval)
					continue
				}
				resources := make(map[string]struct{})
				for _, v := range rs1 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs2 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				allNames := make([]string, 0, len(resources))
				for k := range resources {
					allNames = append(allNames, k)
				}
				sort.Strings(allNames)
				e.cfg.Logger.Info("checking all names", zap.String("namespace", namespace), zap.Strings("names", allNames))
				for _, name := range allNames {
					time.Sleep(100 * time.Millisecond)

					orig, origBody, err := e.GetObject(namespace, from.Kind, name)
					if err != nil {
						return err
					}

					if orig.APIVersion == "" || orig.APIVersion == to.APIVersion {
						e.cfg.Logger.Warn("😁  skipping latest API",
							zap.String("namespace", namespace),
							zap.String("name", name),
							zap.String("current-api-version", orig.APIVersion),
							zap.String("expected-api-version", to.APIVersion),
						)
						continue
					}

					e.cfg.Logger.Warn("😱 😱 😱  found deprecated API!",
						zap.String("namespace", namespace),
						zap.String("name", name),
						zap.String("current-api-version", orig.APIVersion),
						zap.String("expected-api-version", to.APIVersion),
					)

					if err = e.saveKubectlGet(namespace, orig.Kind, name, rbF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlGet(namespace, orig.Kind, name, upF, "\n"); err != nil {
						return err
					}
					patchBody := strings.Replace(
						string(origBody),
						"apiVersion: "+orig.APIVersion+"\n",
						"apiVersion: "+to.APIVersion+"\n",
						1,
					)

					origYAMLPath, err := e.saveYAML(namespace, orig.APIVersion, orig.Kind, name, ".original.yaml", origBody)
					if err != nil {
						return err
					}
					patchYAMLPath, err := e.saveYAML(namespace, to.APIVersion, to.Kind, name, ".patch.yaml", []byte(patchBody))
					if err != nil {
						return err
					}

					if err = e.saveKubectlApply(origYAMLPath, rbF, "\n\n"); err != nil {
						return err
					}
					if err = e.saveKubectlConvert(namespace, from.Kind, to.APIVersion, name, upF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlApply(patchYAMLPath, upF, "\n\n"); err != nil {
						return err
					}
				}
			}

		case from.APIVersion == "extensions/v1beta1" && from.Kind == "ReplicaSet":
			for _, namespace := range namespaces {
				e.cfg.Logger.Info("checking",
					zap.String("from-api-version", from.APIVersion),
					zap.String("from-kind", from.Kind),
					zap.String("to-api-version", to.APIVersion),
					zap.String("to-kind", to.Kind),
					zap.String("namespace", namespace),
				)

				rs1, err := e.ListExtensionsV1beta1ReplicaSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs2, err := e.ListAppsV1ReplicaSets(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}

				if len(rs1) == 0 && len(rs2) == 0 {
					e.cfg.Logger.Info("😁 😁 😁  skipping; no resource found",
						zap.String("from-api-version", from.APIVersion),
						zap.String("from-kind", from.Kind),
						zap.String("namespace", namespace),
					)
					time.Sleep(e.cfg.ListInterval)
					continue
				}
				resources := make(map[string]struct{})
				for _, v := range rs1 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs2 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				allNames := make([]string, 0, len(resources))
				for k := range resources {
					allNames = append(allNames, k)
				}
				sort.Strings(allNames)
				e.cfg.Logger.Info("checking all names", zap.String("namespace", namespace), zap.Strings("names", allNames))
				for _, name := range allNames {
					time.Sleep(100 * time.Millisecond)

					orig, origBody, err := e.GetObject(namespace, from.Kind, name)
					if err != nil {
						return err
					}

					if orig.APIVersion == "" || orig.APIVersion == to.APIVersion {
						e.cfg.Logger.Warn("😁  skipping latest API",
							zap.String("namespace", namespace),
							zap.String("name", name),
							zap.String("current-api-version", orig.APIVersion),
							zap.String("expected-api-version", to.APIVersion),
						)
						continue
					}

					e.cfg.Logger.Warn("😱 😱 😱  found deprecated API!",
						zap.String("namespace", namespace),
						zap.String("name", name),
						zap.String("current-api-version", orig.APIVersion),
						zap.String("expected-api-version", to.APIVersion),
					)

					if err = e.saveKubectlGet(namespace, orig.Kind, name, rbF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlGet(namespace, orig.Kind, name, upF, "\n"); err != nil {
						return err
					}
					patchBody := strings.Replace(
						string(origBody),
						"apiVersion: "+orig.APIVersion+"\n",
						"apiVersion: "+to.APIVersion+"\n",
						1,
					)

					origYAMLPath, err := e.saveYAML(namespace, orig.APIVersion, orig.Kind, name, ".original.yaml", origBody)
					if err != nil {
						return err
					}
					patchYAMLPath, err := e.saveYAML(namespace, to.APIVersion, to.Kind, name, ".patch.yaml", []byte(patchBody))
					if err != nil {
						return err
					}

					if err = e.saveKubectlApply(origYAMLPath, rbF, "\n\n"); err != nil {
						return err
					}
					if err = e.saveKubectlConvert(namespace, from.Kind, to.APIVersion, name, upF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlApply(patchYAMLPath, upF, "\n\n"); err != nil {
						return err
					}
				}
			}

		case from.APIVersion == "extensions/v1beta1" && from.Kind == "NetworkPolicy":
			for _, namespace := range namespaces {
				e.cfg.Logger.Info("checking",
					zap.String("from-api-version", from.APIVersion),
					zap.String("from-kind", from.Kind),
					zap.String("to-api-version", to.APIVersion),
					zap.String("to-kind", to.Kind),
					zap.String("namespace", namespace),
				)

				rs1, err := e.ListExtensionsV1beta1NetworkPolicies(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}
				time.Sleep(e.cfg.ListInterval)
				rs2, err := e.ListNetworkingV1NetworkPolicies(namespace, e.cfg.ListBatch, e.cfg.ListInterval)
				if err != nil {
					return err
				}

				if len(rs1) == 0 && len(rs2) == 0 {
					e.cfg.Logger.Info("😁 😁 😁  skipping; no resource found",
						zap.String("from-api-version", from.APIVersion),
						zap.String("from-kind", from.Kind),
						zap.String("namespace", namespace),
					)
					time.Sleep(e.cfg.ListInterval)
					continue
				}
				resources := make(map[string]struct{})
				for _, v := range rs1 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				for _, v := range rs2 {
					resources[v.ObjectMeta.Name] = struct{}{}
				}
				allNames := make([]string, 0, len(resources))
				for k := range resources {
					allNames = append(allNames, k)
				}
				sort.Strings(allNames)
				e.cfg.Logger.Info("checking all names", zap.String("namespace", namespace), zap.Strings("names", allNames))
				for _, name := range allNames {
					time.Sleep(100 * time.Millisecond)

					orig, origBody, err := e.GetObject(namespace, from.Kind, name)
					if err != nil {
						return err
					}

					if orig.APIVersion == "" || orig.APIVersion == to.APIVersion {
						e.cfg.Logger.Warn("😁  skipping latest API",
							zap.String("namespace", namespace),
							zap.String("name", name),
							zap.String("current-api-version", orig.APIVersion),
							zap.String("expected-api-version", to.APIVersion),
						)
						continue
					}

					e.cfg.Logger.Warn("😱 😱 😱  found deprecated API!",
						zap.String("namespace", namespace),
						zap.String("name", name),
						zap.String("current-api-version", orig.APIVersion),
						zap.String("expected-api-version", to.APIVersion),
					)

					if err = e.saveKubectlGet(namespace, orig.Kind, name, rbF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlGet(namespace, orig.Kind, name, upF, "\n"); err != nil {
						return err
					}
					patchBody := strings.Replace(
						string(origBody),
						"apiVersion: "+orig.APIVersion+"\n",
						"apiVersion: "+to.APIVersion+"\n",
						1,
					)

					origYAMLPath, err := e.saveYAML(namespace, orig.APIVersion, orig.Kind, name, ".original.yaml", origBody)
					if err != nil {
						return err
					}
					patchYAMLPath, err := e.saveYAML(namespace, to.APIVersion, to.Kind, name, ".patch.yaml", []byte(patchBody))
					if err != nil {
						return err
					}

					if err = e.saveKubectlApply(origYAMLPath, rbF, "\n\n"); err != nil {
						return err
					}
					if err = e.saveKubectlConvert(namespace, from.Kind, to.APIVersion, name, upF, "\n"); err != nil {
						return err
					}
					if err = e.saveKubectlApply(patchYAMLPath, upF, "\n\n"); err != nil {
						return err
					}
				}
			}

		case from.APIVersion == "extensions/v1beta1" && from.Kind == "PodSecurityPolicy":
			e.cfg.Logger.Info("checking",
				zap.String("from-api-version", from.APIVersion),
				zap.String("from-kind", from.Kind),
				zap.String("to-api-version", to.APIVersion),
				zap.String("to-kind", to.Kind),
			)

			rs1, err := e.ListExtensionsV1beta1PodSecurityPolicies(e.cfg.ListBatch, e.cfg.ListInterval)
			if err != nil {
				return err
			}
			time.Sleep(e.cfg.ListInterval)
			rs2, err := e.ListPolicyV1beta1PodSecurityPolicies(e.cfg.ListBatch, e.cfg.ListInterval)
			if err != nil {
				return err
			}

			if len(rs1) == 0 && len(rs2) == 0 {
				e.cfg.Logger.Info("😁 😁 😁  skipping; no resource found",
					zap.String("from-api-version", from.APIVersion),
					zap.String("from-kind", from.Kind),
				)
				time.Sleep(e.cfg.ListInterval)
				continue
			}
			resources := make(map[string]struct{})
			for _, v := range rs1 {
				resources[v.ObjectMeta.Name] = struct{}{}
			}
			for _, v := range rs2 {
				resources[v.ObjectMeta.Name] = struct{}{}
			}
			allNames := make([]string, 0, len(resources))
			for k := range resources {
				allNames = append(allNames, k)
			}
			sort.Strings(allNames)
			e.cfg.Logger.Info("checking all names", zap.Strings("names", allNames))
			for _, name := range allNames {
				time.Sleep(100 * time.Millisecond)

				orig, origBody, err := e.GetObject("", from.Kind, name)
				if err != nil {
					return err
				}

				if orig.APIVersion == "" || orig.APIVersion == to.APIVersion {
					e.cfg.Logger.Warn("😁  skipping latest API",
						zap.String("name", name),
						zap.String("current-api-version", orig.APIVersion),
						zap.String("expected-api-version", to.APIVersion),
					)
					continue
				}

				e.cfg.Logger.Warn("😱 😱 😱  found deprecated API!",
					zap.String("name", name),
					zap.String("current-api-version", orig.APIVersion),
					zap.String("expected-api-version", to.APIVersion),
				)

				if err = e.saveKubectlGet("", orig.Kind, name, rbF, "\n"); err != nil {
					return err
				}
				if err = e.saveKubectlGet("", orig.Kind, name, upF, "\n"); err != nil {
					return err
				}
				patchBody := strings.Replace(
					string(origBody),
					"apiVersion: "+orig.APIVersion+"\n",
					"apiVersion: "+to.APIVersion+"\n",
					1,
				)

				origYAMLPath, err := e.saveYAML("", orig.APIVersion, orig.Kind, name, ".original.yaml", origBody)
				if err != nil {
					return err
				}
				patchYAMLPath, err := e.saveYAML("", to.APIVersion, to.Kind, name, ".patch.yaml", []byte(patchBody))
				if err != nil {
					return err
				}

				if err = e.saveKubectlApply(origYAMLPath, rbF, "\n\n"); err != nil {
					return err
				}
				if err = e.saveKubectlConvert("namespace", from.Kind, to.APIVersion, name, upF, "\n"); err != nil {
					return err
				}
				if err = e.saveKubectlApply(patchYAMLPath, upF, "\n\n"); err != nil {
					return err
				}
			}

		default:
			return fmt.Errorf("upgrade operation not implemented for %q %q", from.APIVersion, from.Kind)
		}
	}

	return nil
}

func createBashScript(p string) (f *os.File, err error) {
	f, err = os.OpenFile(p, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(p)
	}
	if _, err = f.Write([]byte(scriptHeader)); err != nil {
		return nil, err
	}
	return f, err
}

func (e *eks) saveYAML(namespace string, apiVersion string, kind string, name string, sfx string, d []byte) (string, error) {
	if namespace == "" {
		namespace = "all"
	}
	apiVersion = strings.ReplaceAll(apiVersion, "/", "")

	fileName := namespace + "-" + kind + "-" + name + "-" + apiVersion + sfx
	fpath := filepath.Join(e.cfg.Dir, fileName)

	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0444)
	if err != nil {
		f, err = os.Create(fpath)
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.Write(d)
	e.cfg.Logger.Info("wrote", zap.String("path", fpath))
	return fpath, err
}

func (e *eks) saveKubectlGet(namespace string, kind string, name string, f *os.File, end string) error {
	ss := []string{e.cfg.KubectlPath, "--kubeconfig=" + e.cfg.KubeConfigPath}
	if namespace != "" {
		ss = append(ss, "--namespace="+namespace)
	}
	ss = append(ss, "get", strings.ToLower(kind), name, "-o=yaml")
	_, err := f.Write([]byte(strings.Join(ss, " ") + end))
	e.cfg.Logger.Info("wrote kubectl get command", zap.String("path", f.Name()))
	return err
}

func (e *eks) saveKubectlConvert(namespace string, kind string, targetAPIVer string, name string, f *os.File, end string) error {
	ss := []string{e.cfg.KubectlPath, "--kubeconfig=" + e.cfg.KubeConfigPath}
	if namespace != "" {
		ss = append(ss, "--namespace="+namespace)
	}
	ss = append(ss, "get", strings.ToLower(kind), name, "-o=yaml", ">", "/tmp/"+namespace+"-"+kind+"-"+name+".yaml")
	ss = append(ss, "&&", e.cfg.KubectlPath, "convert", "--output-version="+targetAPIVer, "-f", "/tmp/"+namespace+"-"+kind+"-"+name+".yaml")
	_, err := f.Write([]byte(strings.Join(ss, " ") + end))
	e.cfg.Logger.Info("wrote kubectl convert command", zap.String("path", f.Name()))
	return err
}

func (e *eks) saveKubectlApply(p string, f *os.File, end string) error {
	ss := []string{e.cfg.KubectlPath, "--kubeconfig=" + e.cfg.KubeConfigPath, "apply", "-f", p}
	_, err := f.Write([]byte(strings.Join(ss, " ") + end))
	e.cfg.Logger.Info("wrote kubectl apply command", zap.String("path", f.Name()))
	return err
}
