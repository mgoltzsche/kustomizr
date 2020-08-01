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
	apiVersionConfigKubernetesIO = "config.kubernetes.io"
	apiVersionConfigK8sIO        = "config.k8s.io"
	annotationIndex              = apiVersionConfigKubernetesIO + "/index"
	annotationLocalConfig        = apiVersionConfigK8sIO + "/local-config"
	kustomizationFile            = "kustomization.yaml"
	inventoryTemplateFile        = "inventory-template.yaml"
	kindKustomization            = "Kustomization"
	apiVersionKustomization      = "kustomize.config.k8s.io/v1beta1"
)

func kptAnnotationMatcher(name string) func(map[string]string) string {
	name1 := apiVersionConfigKubernetesIO + "/" + name
	name2 := apiVersionConfigK8sIO + "/" + name
	return func(a map[string]string) string {
		v1 := a[name1]
		v2 := a[name2]
		if v1 != "" {
			return v1
		}
		return v2
	}
}

var (
	annotationPath     = kptAnnotationMatcher("path")
	annotationFunction = kptAnnotationMatcher("function")
)

func main() {
	path := "."
	config := configMap{}
	resourceList := &framework.ResourceList{FunctionConfig: &config}
	cmd := framework.Command(resourceList, func() error {
		if len(resourceList.Items) == 0 {
			return fmt.Errorf("no resources provided")
		}
		tmpDir, err := ioutil.TempDir("", "")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		kustomizationFile := filepath.Join(path, "kustomization.yaml")
		kustomizationFileFound := false
		preservedResources := []*yaml.RNode{}
		lastNamespace := ""

		for _, item := range resourceList.Items {
			meta, err := item.GetMeta()
			if err != nil {
				return err
			}
			if meta.Annotations == nil {
				continue
			}
			itemPath := annotationPath(meta.Annotations)
			if itemPath == "" || filepath.IsAbs(itemPath) {
				continue
			}
			if itemPath == kustomizationFile {
				kustomizationFileFound = true
			}

			itemFileName := filepath.Base(itemPath)
			if itemFileName == kustomizationFile {
				if item.Field(yaml.APIVersionField) == nil {
					err = item.PipeE(yaml.FieldSetter{Name: yaml.APIVersionField, StringValue: apiVersionKustomization})
					if err != nil {
						return err
					}
				}
				if item.Field(yaml.KindField) == nil {
					err = item.PipeE(yaml.FieldSetter{Name: yaml.KindField, StringValue: kindKustomization})
					if err != nil {
						return err
					}
				}

				trueStr := yaml.NewScalarRNode("true")
				trueStr.YNode().Style = yaml.SingleQuotedStyle
				err = item.PipeE(yaml.LookupCreate(yaml.ScalarNode, yaml.MetadataField, yaml.AnnotationsField, annotationLocalConfig), yaml.FieldSetter{Value: trueStr, OverrideStyle: true})
				if err != nil {
					return err
				}
				preserved := yaml.MustParse(item.MustString())
				preservedResources = append(preservedResources, preserved)

				// Workaround for https://github.com/GoogleContainerTools/kpt/issues/755
				nameNode := yaml.NewScalarRNode("kustomization")
				nameNode.YNode().Style = yaml.SingleQuotedStyle
				err = preserved.PipeE(yaml.LookupCreate(yaml.ScalarNode, yaml.MetadataField, yaml.NameField), yaml.FieldSetter{Value: nameNode})
				if err != nil {
					return err
				}
				defer func() {
					nsNode := yaml.NewScalarRNode(lastNamespace)
					nsNode.YNode().Style = yaml.SingleQuotedStyle
					preserved.PipeE(yaml.LookupCreate(yaml.ScalarNode, yaml.MetadataField, yaml.NamespaceField), yaml.FieldSetter{Value: nsNode})
				}()

				err = item.PipeE(yaml.FieldSetter{Name: yaml.MetadataField, Value: nil})
				if err != nil {
					return err
				}
				err = item.PipeE(yaml.FieldSetter{Name: yaml.APIVersionField, Value: yaml.NullNode()})
				if err != nil {
					return err
				}
				err = item.PipeE(yaml.FieldSetter{Name: yaml.KindField, Value: yaml.NullNode()})
				if err != nil {
					return err
				}
			} else if annotationFunction(meta.Annotations) != "" || itemFileName == inventoryTemplateFile {
				preservedResources = append(preservedResources, item)
			}

			tmpItemPath := filepath.Join(tmpDir, itemPath)
			data, err := item.String()
			if err != nil {
				return err
			}
			err = os.MkdirAll(filepath.Dir(tmpItemPath), 0755)
			if err != nil {
				return err
			}
			err = writeFile(tmpItemPath, []byte("---\n"+data))
			if err != nil {
				return err
			}
		}
		if !kustomizationFileFound {
			return fmt.Errorf("kustomization file %s is not among function input", kustomizationFile)
		}

		kArgs := kustomizeArgs(path, config.Data)
		var transformed []*yaml.RNode
		transformed, lastNamespace, err = buildKustomization(tmpDir, kArgs)
		if err != nil {
			return err
		}
		resourceList.Items = transformed
		resourceList.Items = append(resourceList.Items, preservedResources...)
		return nil
	})
	cmd.Flags().StringVar(&path, flagPath, path, "path to kustomization")
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type configMap struct {
	Data map[string]string
}

func kustomizeArgs(path string, config map[string]string) []string {
	a := make([]string, 2, len(config)+2)
	a[0] = "build"
	a[1] = path
	for k, v := range config {
		if k != flagPath {
			a = append(a, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return a
}

func buildKustomization(tmpDir string, kustomizeArgs []string) ([]*yaml.RNode, string, error) {
	cmd := exec.Command("kustomize", kustomizeArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Dir = tmpDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, "", fmt.Errorf("kustomize build: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	// Parse output
	r := []*yaml.RNode{}
	d := yaml.NewDecoder(&stdout)
	lastNamespace := ""
	for {
		v := yaml.Node{}
		o := yaml.NewRNode(&v)
		err = d.Decode(&v)
		if err != nil {
			break
		}
		err = o.PipeE(yaml.Lookup(yaml.MetadataField, yaml.AnnotationsField), yaml.FieldSetter{Name: annotationIndex, StringValue: strconv.Itoa(len(r))})
		if err != nil {
			return nil, "", err
		}
		m, _ := o.GetMeta()
		if m.Namespace != "" {
			lastNamespace = m.Namespace
		}
		r = append(r, o)
	}
	if err != nil && err != io.EOF {
		return nil, "", fmt.Errorf("parse kustomize output: %w", err)
	}
	return r, lastNamespace, nil
}

func writeFile(file string, data []byte) (err error) {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer func() {
		if e := f.Close(); e != nil && err == nil {
			err = e
		}
	}()
	_, err = f.Write(data)
	return
}
