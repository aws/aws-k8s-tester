package frameworkext

import (
	"context"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

// ApplyFiles creates Kubernetes objects contained in manifest file(s), in a manner similar to `kubectl apply -f`
// Multiple objects may be in each manifest file.
// The manifest files are processed in order.
func ApplyFiles(restConfig *rest.Config, manifestFiles []string) error {
	for _, manifestFile := range manifestFiles {
		if err := ApplyFile(restConfig, manifestFile); err != nil {
			return err
		}
	}
	return nil
}

// ApplyFile creates Kubernetes objects contained in a manifest file, in a manner similar to `kubectl apply -f`
// Multiple objects may be in the manifest file.
func ApplyFile(restConfig *rest.Config, manifestFile string) error {
	f, err := os.Open(manifestFile)
	if err != nil {
		return err
	}
	objs, err := decoder.DecodeAll(context.TODO(), f)
	if err != nil {
		return err
	}
	return processObjects(restConfig, objs, func(client *resource.Helper, obj k8s.Object) error {
		namespace, err := meta.NewAccessor().Namespace(obj)
		if err != nil {
			return err
		}
		if namespace == "" {
			namespace = "default"
		}
		_, err = client.Create(namespace, false, obj)
		return err
	})
}

// DeleteFiles deletes Kubernetes objects contained in manifest file(s), in a manner similar to `kubectl delete -f`
// Multiple objects may be in each manifest file.
// The manifest files are processed in order.
func DeleteFiles(restConfig *rest.Config, manifestFiles []string) error {
	for _, manifestFile := range manifestFiles {
		if err := DeleteFile(restConfig, manifestFile); err != nil {
			return err
		}
	}
	return nil
}

// DeleteFile deletes Kubernetes objects contained in a manifest file, in a manner similar to `kubectl delete -f`
// Multiple objects may be in the manifest file.
func DeleteFile(restConfig *rest.Config, manifestFile string) error {
	f, err := os.Open(manifestFile)
	if err != nil {
		return err
	}
	objs, err := decoder.DecodeAll(context.TODO(), f)
	if err != nil {
		return err
	}
	return processObjects(restConfig, objs, func(client *resource.Helper, obj k8s.Object) error {
		name, err := meta.NewAccessor().Name(obj)
		if err != nil {
			return err
		}
		namespace, err := meta.NewAccessor().Namespace(obj)
		if err != nil {
			return err
		}
		if namespace == "" {
			namespace = "default"
		}
		_, err = client.Delete(namespace, name)
		return err
	})
}

// processObjects applies a processFunc to each object, supplying it a dynamically-typed client appropriate for the object
func processObjects(restConfig *rest.Config, objs []k8s.Object, processFunc func(client *resource.Helper, obj k8s.Object) error) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)
	for _, obj := range objs {
		client, err := newResourceHelper(restConfig, rm, obj)
		if err != nil {
			return err
		}
		processFunc(client, obj)
	}
	return nil
}

func newResourceHelper(restConfig *rest.Config, rm meta.RESTMapper, obj runtime.Object) (*resource.Helper, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, err
	}
	gv := mapping.GroupVersionKind.GroupVersion()
	restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv
	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	return resource.NewHelper(restClient, mapping), nil
}
