package installable

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlUtil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Yaml struct {
	k8sClient   client.Client
	yamlGlob    string
	displayName string
}

func NewYaml(k8sClient client.Client, yamlGlob string, displayName string) *Yaml {
	return &Yaml{
		k8sClient:   k8sClient,
		yamlGlob:    yamlGlob,
		displayName: displayName,
	}
}

func (y *Yaml) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	matchedFiles, err := filepath.Glob(y.yamlGlob)
	if err != nil {
		return Result{}, err
	}

	if len(matchedFiles) == 0 {
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("no matches found for pattern %s", y.yamlGlob),
		}, nil
	}

	for _, file := range matchedFiles {
		result, err := y.installFile(ctx, file)
		if err != nil {
			return Result{}, err
		}

		if result.State != ResultStateSuccess {
			eventRecorder.Event(EventWarning, "InstallableFailed", fmt.Sprintf("Installable %s failed", y.displayName))
			return result, nil
		}
	}

	eventRecorder.Event(EventNormal, "InstallableDeployed", fmt.Sprintf("Installable %s deployed", y.displayName))
	return Result{
		State:   ResultStateSuccess,
		Message: fmt.Sprintf("%s installed successfully", y.displayName),
	}, nil
}

func (y *Yaml) installFile(ctx context.Context, yamlFilePath string) (Result, error) {
	yamlBytes, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to read file %s: %s", yamlFilePath, err.Error()),
		}, nil
	}

	objects, err := parseToUnstructuredObjects(string(yamlBytes))
	if err != nil {
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to parse file to objects: %s", err),
		}, nil
	}

	for _, obj := range objects {
		err := y.createOrUpdate(ctx, obj)
		if err != nil {
			return Result{}, fmt.Errorf("failed to create/update %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	return Result{
		State: ResultStateSuccess,
	}, nil
}

func (y *Yaml) createOrUpdate(ctx context.Context, unstructuredObj *unstructured.Unstructured) error {
	var partialObj metav1.PartialObjectMetadata
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &partialObj)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured %s/%s to partialObj: %w", unstructuredObj.GetNamespace(), unstructuredObj.GetName(), err)
	}

	err = y.k8sClient.Get(ctx, client.ObjectKeyFromObject(&partialObj), &partialObj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return y.k8sClient.Create(ctx, unstructuredObj)
		}
		return fmt.Errorf("failed to get existing object %s/%s: %w", partialObj.GetNamespace(), partialObj.GetName(), err)
	}

	err = setResourceVersion(unstructuredObj, partialObj.GetResourceVersion())
	if err != nil {
		return fmt.Errorf("failed to set resource version for %s/%s: %w", unstructuredObj.GetNamespace(), unstructuredObj.GetName(), err)
	}

	err = y.k8sClient.Update(ctx, unstructuredObj)
	if err != nil {
		return fmt.Errorf("failed to update existing object: %w", err)
	}

	return nil
}

func parseToUnstructuredObjects(yamlContent string) ([]*unstructured.Unstructured, error) {
	objects := []*unstructured.Unstructured{}
	reader := yamlUtil.NewYAMLReader(bufio.NewReader(strings.NewReader(yamlContent)))
	for {
		rawBytes, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return objects, nil
			}

			return nil, fmt.Errorf("invalid YAML doc: %w", err)
		}

		rawBytes = bytes.TrimSpace(rawBytes)
		if len(rawBytes) == 0 || bytes.Equal(rawBytes, []byte("null")) {
			continue
		}

		unstructuredObj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(rawBytes, &unstructuredObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %q to unstructured: %w", string(rawBytes), err)
		}

		if len(unstructuredObj.Object) == 0 {
			continue
		}

		objects = append(objects, &unstructuredObj)
	}
}

func setResourceVersion(unstructuredObj *unstructured.Unstructured, resourceVersion string) error {
	metaAccessor, err := meta.Accessor(unstructuredObj)
	if err != nil {
		return fmt.Errorf("failed to get meta accessor for %s/%s: %w", unstructuredObj.GetNamespace(), unstructuredObj.GetName(), err)
	}
	metaAccessor.SetResourceVersion(resourceVersion)

	return nil
}
