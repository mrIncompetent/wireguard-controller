package cniconfig

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"text/template"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

type tplData struct {
	PodCIDR     string
	NodePodCIDR string
	MTU         int
}

func (r *Reconciler) writeCNIConfig(log *zap.Logger, node *corev1.Node, mtu int) error {
	_, nodePodCidr, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return fmt.Errorf("unable to parse node pod cidr: %w", err)
	}

	data := tplData{
		PodCIDR:     r.podNet.String(),
		NodePodCIDR: nodePodCidr.String(),
		MTU:         mtu,
	}

	files, err := ioutil.ReadDir(path.Clean(r.cni.TemplateDir))
	if err != nil {
		return fmt.Errorf("unable to list template files: %w", err)
	}

	for _, file := range files {
		sourceFilename := path.Join(r.cni.TemplateDir, file.Name())
		// ioutil.ReadDir uses LState which does not follow symlinks - So symlinked directories will return false on IsDir
		fileInfo, err := os.Stat(sourceFilename)
		if err != nil {
			return fmt.Errorf("unable to check file '%s': %w", sourceFilename, err)
		}

		if fileInfo.IsDir() {
			continue
		}

		targetFilename := path.Join(r.cni.TargetDir, fileInfo.Name())
		if err := templateFile(log, sourceFilename, targetFilename, data); err != nil {
			return fmt.Errorf("unable to template file '%s': %w", sourceFilename, err)
		}
	}

	return nil
}

func templateFile(parentLog *zap.Logger, sourceFilename, targetFilename string, data tplData) error {
	log := parentLog.With(
		zap.String("template_source", sourceFilename),
		zap.String("template_target", targetFilename),
	)

	content, err := ioutil.ReadFile(sourceFilename)
	if err != nil {
		return fmt.Errorf("reading file failed: %w", err)
	}

	log.Debug("successfully read template file")

	tpl, err := template.New(path.Base(sourceFilename)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	log.Debug("successfully parsed template file")

	output := &bytes.Buffer{}
	if err := tpl.Execute(output, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	log.Debug("successfully executed the template")

	currentContent, err := ioutil.ReadFile(targetFilename)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading file failed: %w", err)
	}

	if bytes.Equal(currentContent, output.Bytes()) {
		log.Debug("Not writing CNI config as its already up to date")

		return nil
	}

	log.Info("CNI config does not match desired config, will override it")

	if err := ioutil.WriteFile(targetFilename, output.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write CNI file: %w", err)
	}

	log.Info("Successfully wrote CNI config")

	return nil
}
