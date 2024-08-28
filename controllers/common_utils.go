package controllers

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	yamlUtil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/yaml"
)

type RateLimiter struct {
	Burst           int
	Frequency       int
	BaseDelay       time.Duration
	FailureMaxDelay time.Duration
}

const (
	requeueInterval = time.Hour * 1
	finalizer       = "sample.kyma-project.io/finalizer"
	debugLogLevel   = 2
	fieldOwner      = "sample.kyma-project.io/owner"
)

func (r *CFAPIReconciler) installOneGlob(ctx context.Context, pattern string) error {
	logger := log.FromContext(ctx)
	logger.Info("Installing", "glob", pattern)
	resources, err := loadOneGlob(pattern)

	if err != nil {
		return err
	}

	for _, obj := range resources.Items {
		if err := r.ssa(ctx, obj); err != nil && !errors2.IsAlreadyExists(err) {
			logger.Error(err, "error during installation of resources")
			return err
		}
	}
	return nil
}

func loadOneGlob(pattern string) (*ManifestResources, error) {
	filename, err := findOneGlob(pattern)
	if err != nil {
		return nil, err
	}
	return loadFile(filename)
}

func loadFile(file string) (*ManifestResources, error) {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parseManifestStringToObjects(string(fileBytes))
}

func findOneGlob(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("Ambiguous file glob %s, found more than one file", pattern)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("No file glob found %s", pattern)
	}
	return matches[0], nil
}

func loadOneYaml(pattern string) (map[string]any, error) {
	file, err := findOneGlob(pattern)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	content := make(map[string]any, 50)
	err = yaml.Unmarshal(data, &content)
	return content, err
}

// parseManifestStringToObjects parses the string of resources into a list of unstructured resources.
func parseManifestStringToObjects(manifest string) (*ManifestResources, error) {
	objects := &ManifestResources{}
	reader := yamlUtil.NewYAMLReader(bufio.NewReader(strings.NewReader(manifest)))
	for {
		rawBytes, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return objects, nil
			}

			return nil, fmt.Errorf("invalid YAML doc: %w", err)
		}

		rawBytes = bytes.TrimSpace(rawBytes)
		unstructuredObj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(rawBytes, &unstructuredObj); err != nil {
			objects.Blobs = append(objects.Blobs, append(bytes.TrimPrefix(rawBytes, []byte("---\n")), '\n'))
		}

		if len(rawBytes) == 0 || bytes.Equal(rawBytes, []byte("null")) || len(unstructuredObj.Object) == 0 {
			continue
		}

		objects.Items = append(objects.Items, &unstructuredObj)
	}
}

// TemplateRateLimiter implements a rate limiter for a client-go.workqueue.  It has
// both an overall (token bucket) and per-item (exponential) rate limiting.
func TemplateRateLimiter(failureBaseDelay time.Duration, failureMaxDelay time.Duration,
	frequency int, burst int,
) ratelimiter.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(failureBaseDelay, failureMaxDelay),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(frequency), burst)})
}

// getResourcesFromLocalPath returns resources from the dirPath in unstructured format.
// Only one file in .yaml or .yml format should be present in the target directory.
func getResourcesFromLocalPath(dirPath string, logger logr.Logger) (*ManifestResources, error) {
	dirEntries := make([]fs.DirEntry, 0)
	err := filepath.WalkDir(dirPath, func(path string, info fs.DirEntry, err error) error {
		// initial error
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		dirEntries, err = os.ReadDir(dirPath)
		return err
	})
	if err != nil {
		return nil, err
	}

	childCount := len(dirEntries)
	if childCount == 0 {
		logger.V(debugLogLevel).Info(fmt.Sprintf("no yaml file found at file path %s", dirPath))
		return nil, nil
	} else if childCount > 1 {
		logger.V(debugLogLevel).Info(fmt.Sprintf("more than one yaml file found at file path %s", dirPath))
		return nil, nil
	}
	file := dirEntries[0]
	allowedExtns := sets.NewString(".yaml", ".yml")
	if !allowedExtns.Has(filepath.Ext(file.Name())) {
		return nil, nil
	}

	fileBytes, err := os.ReadFile(filepath.Join(dirPath, file.Name()))
	if err != nil {
		return nil, fmt.Errorf("yaml file could not be read %s in dir %s: %w", file.Name(), dirPath, err)
	}
	return parseManifestStringToObjects(string(fileBytes))
}
