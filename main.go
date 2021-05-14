package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	flagPath                     = "path"
	flagOutputPath               = "outputPath"
	apiVersionConfigKubernetesIO = "config.kubernetes.io"
	annotationIndex              = apiVersionConfigKubernetesIO + "/index"
	annotationPath               = apiVersionConfigKubernetesIO + "/path"
	kustomizationFileName        = "kustomization.yaml"
)

func main() {
	pathOption := "."
	outputFileOption := filepath.Join("static", "generated.yaml")
	config := configMap{}
	resourceList := &framework.ResourceList{FunctionConfig: &config}
	cmd := framework.Command(resourceList, func() (err error) {
		fmt.Fprintln(os.Stderr, "# Running kustomize", os.Getenv("KUSTOMIZE_VERSION"))
		outputFileOption = filepath.Clean(outputFileOption)
		copyFromInput := false
		kptDir := pathOption
		kustomizationDir := filepath.Clean(pathOption)
		if !filepath.IsAbs(pathOption) {
			// Prepare temp dir to copy from stdin
			// when project dir is not mounted
			copyFromInput = true
			kptDir, err = ioutil.TempDir("", "")
			if err != nil {
				return err
			}
			defer os.RemoveAll(kptDir)
			kustomizationDir = filepath.Join(kptDir, pathOption)
		}
		filteredItems := make([]*yaml.RNode, 0, len(resourceList.Items))

		for _, item := range resourceList.Items {
			meta, err := item.GetMeta()
			if err != nil {
				return err
			}
			if meta.Annotations == nil {
				continue
			}
			// Exclude resources from previous output or with absolute or unknown path
			itemPath := meta.Annotations[annotationPath]
			if itemPath == "" || filepath.IsAbs(itemPath) || itemPath == outputFileOption {
				continue
			}
			filteredItems = append(filteredItems, item)

			if copyFromInput {
				// Write stdin to temp dir
				transformed := item
				if filepath.Base(itemPath) == kustomizationFileName {
					transformed = yaml.MustParse(transformed.MustString())
					err = transformed.PipeE(yaml.FieldSetter{Name: yaml.MetadataField, Value: nil})
					if err != nil {
						return err
					}
				}
				tmpItemPath := filepath.Join(kptDir, itemPath)
				err = writeFile(tmpItemPath, transformed)
				if err != nil {
					return err
				}
			}
		}

		// Render the kustomization
		kArgs := kustomizeArgs(kustomizationDir, config.Data)
		transformed, err := buildKustomization(outputFileOption, kArgs)
		if err != nil {
			return err
		}

		if filepath.IsAbs(outputFileOption) {
			// Write file to disk (sink) when absolute output path provided
			manifest := ""
			for _, o := range transformed {
				data, err := o.String()
				if err != nil {
					return err
				}
				manifest += "---\n" + data
			}
			err = ioutil.WriteFile(outputFileOption, []byte(manifest), 0644)
			if err != nil {
				return err
			}
		}

		// Add rendered resources to output
		resourceList.Items = append(filteredItems, transformed...)
		return nil
	})

	cmd.Flags().StringVar(&pathOption, flagPath, pathOption, "path to kustomization")
	cmd.Flags().StringVar(&outputFileOption, flagOutputPath, outputFileOption, "output manifest path")

	if err := cmd.Execute(); err != nil {
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
}

type configMap struct {
	Data map[string]string
}

func kustomizeArgs(path string, config map[string]string) []string {
	a := make([]string, 0, len(config)+2)
	a = append(a, "build", path)
	for k, v := range config {
		if k != flagPath && k != flagOutputPath {
			a = append(a, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return a
}

func buildKustomization(outputPath string, kustomizeArgs []string) ([]*yaml.RNode, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("kustomize", kustomizeArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("kustomize build: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	// Parse output
	r := []*yaml.RNode{}
	d := yaml.NewDecoder(&stdout)
	for {
		v := yaml.Node{}
		o := yaml.NewRNode(&v)
		err = d.Decode(&v)
		if err != nil {
			break
		}
		err = setKptAnnotations(o, outputPath, len(r))
		if err != nil {
			break
		}
		r = append(r, o)
	}
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("process kustomize output: %w", err)
	}
	return r, nil
}

func setKptAnnotations(o *yaml.RNode, path string, index int) error {
	// Remove annotations field if empty.
	// This is required because LookupCreate() doesn't create the MappingNode if it exists but is empty.
	err := o.PipeE(yaml.LookupCreate(yaml.MappingNode, yaml.MetadataField), yaml.FieldClearer{Name: yaml.AnnotationsField, IfEmpty: true})
	if err != nil {
		return err
	}
	// Add path annotations
	lookupAnnotations := yaml.LookupCreate(yaml.MappingNode, yaml.MetadataField, yaml.AnnotationsField)
	err = o.PipeE(lookupAnnotations, yaml.FieldSetter{Name: annotationIndex, StringValue: strconv.Itoa(index)})
	if err != nil {
		return err
	}
	err = o.PipeE(lookupAnnotations, yaml.FieldSetter{Name: annotationPath, StringValue: path})
	if err != nil {
		return err
	}
	return nil
}

func writeFile(file string, transformed *yaml.RNode) (err error) {
	data, err := transformed.String()
	if err != nil {
		return
	}
	err = os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer func() {
		if e := f.Close(); e != nil && err == nil {
			err = e
		}
	}()
	_, err = f.Write([]byte("---\n" + data))
	return
}
