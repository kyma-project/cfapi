package installable

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlUtil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type yamlBytesInstaller struct {
	k8sClient client.Client
	yamlBytes []byte
}

func newYamlBytesInstaller(k8sClient client.Client, yamlBytes []byte) *yamlBytesInstaller {
	return &yamlBytesInstaller{
		k8sClient: k8sClient,
		yamlBytes: yamlBytes,
	}
}

func (y *yamlBytesInstaller) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	objects, err := parseToUnstructuredObjects(string(y.yamlBytes))
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

func (y *yamlBytesInstaller) createOrUpdate(ctx context.Context, obj client.Object) error {
	err := y.k8sClient.Create(ctx, obj)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return y.k8sClient.Update(ctx, obj)
		}

		return err
	}

	return nil
}

type YamlFile struct {
	k8sClient    client.Client
	yamlFilePath string
	displayName  string
}

func NewYamlFile(k8sClient client.Client, yamlFilePath string, displayName string) *YamlFile {
	return &YamlFile{
		k8sClient:    k8sClient,
		yamlFilePath: yamlFilePath,
		displayName:  displayName,
	}
}

func (y *YamlFile) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	yamlBytes, err := os.ReadFile(y.yamlFilePath)
	if err != nil {
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to read file %s: %s", y.yamlFilePath, err.Error()),
		}, nil
	}

	result, err := newYamlBytesInstaller(y.k8sClient, yamlBytes).Install(ctx, config, eventRecorder)
	if result.State == ResultStateSuccess {
		eventRecorder.Event("InstallableDeployed", "Installable %s deployed", y.displayName)
	}

	return result, err
}

type YamlTemplate struct {
	k8sClient            client.Client
	yamlTemplateFilePath string
	displayName          string
}

func NewYamlTemplate(k8sClient client.Client, yamlTemplateFilePath string, displayName string) *YamlTemplate {
	return &YamlTemplate{
		k8sClient:            k8sClient,
		yamlTemplateFilePath: yamlTemplateFilePath,
		displayName:          displayName,
	}
}

func (y *YamlTemplate) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	tmpl, err := template.ParseFiles(y.yamlTemplateFilePath)
	if err != nil {
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to parse the template %s: %w", y.yamlTemplateFilePath, err.Error()),
		}, nil
	}

	buf := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(buf, tmpl.Name(), config)
	if err != nil {
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to execute template %s: %w", y.yamlTemplateFilePath, err.Error()),
		}, nil
	}

	result, err := newYamlBytesInstaller(y.k8sClient, buf.Bytes()).Install(ctx, config, eventRecorder)
	if result.State == ResultStateSuccess {
		eventRecorder.Event("InstallableDeployed", "Installable %s deployed", y.displayName)
	}

	return result, err
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
