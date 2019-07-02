package cni_config

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type tplData struct {
	PodCIDR     string
	NodePodCIDR string
	MTU         int
}

func (r *Reconciler) writeCNIConfig(parentLog *zap.Logger, mtu int) error {
	log := parentLog.Named("cni_config_writer")
	data := tplData{
		PodCIDR:     r.podCidrNet.String(),
		NodePodCIDR: r.nodePodCidrNet.String(),
		MTU:         mtu,
	}

	files, err := ioutil.ReadDir(path.Clean(r.cni.TemplateDir))
	if err != nil {
		return errors.Wrap(err, "unable to list template files")
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		sourceFilename := path.Join(r.cni.TemplateDir, file.Name())
		targetFilename := path.Join(r.cni.TargetDir, file.Name())
		if err := templateFile(log, sourceFilename, targetFilename, data); err != nil {
			return errors.Wrapf(err, "unable to template file '%s'", sourceFilename)
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
		return err
	}
	log.Debug("successfully read template file")

	tpl, err := template.New(path.Base(sourceFilename)).Parse(string(content))
	if err != nil {
		return err
	}
	log.Debug("successfully parsed template file")

	output := &bytes.Buffer{}
	if err := tpl.Execute(output, data); err != nil {
		return err
	}
	log.Debug("successfully executed the template")

	currentContent, err := ioutil.ReadFile(targetFilename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if bytes.Compare(currentContent, output.Bytes()) == 0 {
		log.Debug("Not writing CNI config as its already up to date")
		return nil
	}
	log.Info("CNI config does not match desired config, will override it")

	if err := ioutil.WriteFile(targetFilename, output.Bytes(), 0644); err != nil {
		return err
	}
	log.Info("Successfully wrote CNI config")

	return nil
}
