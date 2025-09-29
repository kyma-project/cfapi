package helpers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiyaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func DeleteYamlFilesInDir(ctx context.Context, dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") {
			err = DeleteYamlFile(ctx, fmt.Sprintf("%s/%s", dirPath, file.Name()))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteYamlFile(ctx context.Context, yamlFilePath string) error {
	yamlDocs, err := parseYamlIntoDocs(yamlFilePath)
	if err != nil {
		return err
	}

	yamlDocs, err = filterComments(yamlDocs)
	if err != nil {
		return err
	}

	for _, doc := range yamlDocs {
		resourceClient, obj, err := resourceClientFor(doc)
		if err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			return fmt.Errorf("failed to create resource client for %s: %w", string(doc), err)
		}

		deleteForeground := metav1.DeletePropagationForeground
		err = resourceClient.Delete(ctx, obj.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deleteForeground,
		})
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting object %s failed: %w", obj.GetName(), err)
		}

		_, err = resourceClient.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("object %s/%s still exists", obj.GetNamespace(), obj.GetName())
		}

		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get object %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}

func parseYamlIntoDocs(yamlFilePath string) ([][]byte, error) {
	yamlData, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return nil, err
	}

	yamlDocs := bytes.Split(yamlData, []byte("---"))
	return yamlDocs, nil
}

func filterComments(yamlDocs [][]byte) ([][]byte, error) {
	result := [][]byte{}

	for _, doc := range yamlDocs {
		obj := map[string]any{}
		err := yaml.Unmarshal(doc, &obj)
		if err != nil {
			return nil, err
		}

		if len(obj) == 0 {
			continue
		}

		marshalledDoc, err := yaml.Marshal(obj)
		if err != nil {
			return nil, err
		}

		result = append(result, marshalledDoc)
	}

	return result, nil
}

func resourceClientFor(yamlDoc []byte) (dynamic.ResourceInterface, *unstructured.Unstructured, error) {
	dynClient, err := createDynamicClient()
	if err != nil {
		return nil, nil, err
	}

	decoder := apiyaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

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
