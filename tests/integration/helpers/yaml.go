package helpers

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func DeleteYamlFile(ctx context.Context, yamlFilePath string) error {
	yamlDocs, err := parseYamlIntoDocs(yamlFilePath)
	if err != nil {
		return err
	}

	for _, doc := range yamlDocs {
		resourceClient, obj, err := resourceClientFor(doc)
		if err != nil {
			return err
		}

		deleteForeground := metav1.DeletePropagationForeground
		err = resourceClient.Delete(ctx, obj.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deleteForeground,
		})
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		_, err = resourceClient.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("object %s/%s still exists", obj.GetNamespace(), obj.GetName())
		}

		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

func parseYamlIntoDocs(yamlFilePath string) ([][]byte, error) {
	yamlData, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return nil, err
	}

	yamlDocs := [][]byte{}
	splitDocs := bytes.Split(yamlData, []byte("---\napiVersion"))
	for i, doc := range splitDocs {
		// Only restore "apiVersion" for the second and subsequent elements (the first element does not start with the separator)
		if i != 0 {
			doc = append([]byte("apiVersion"), doc...)
		}

		yamlDocs = append(yamlDocs, doc)
	}

	return yamlDocs, nil
}

func resourceClientFor(yamlDoc []byte) (dynamic.ResourceInterface, *unstructured.Unstructured, error) {
	dynClient, err := createDynamicClient()
	if err != nil {
		return nil, nil, err
	}

	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	obj := &unstructured.Unstructured{}
	_, gvk, err := decoder.Decode(yamlDoc, nil, obj)
	if err != nil {
		return nil, nil, err
	}

	mapper, err := createDynamicRestMapper()
	if err != nil {
		return nil, nil, err
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, nil, err
	}

	return dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()), obj, nil
}

func createDynamicClient() (dynamic.Interface, error) {
	config, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func createDynamicRestMapper() (meta.RESTMapper, error) {
	config, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, err
	}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}

	return apiutil.NewDynamicRESTMapper(config, httpClient)
}
