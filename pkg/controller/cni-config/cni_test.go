package cniconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	customlog "github.com/mrincompetent/wireguard-controller/pkg/log"
	testhelper "github.com/mrincompetent/wireguard-controller/pkg/test"

	"go.uber.org/zap/zapcore"
)

func TestTemplateFile(t *testing.T) {
	tests := []struct {
		name           string
		data           tplData
		tpl            string
		expectedResult string
		expectedErr    error
		expectedLog    string
	}{
		{
			name: "simple template",
			tpl:  "Foo {{ .PodCIDR }} Bar",
			data: tplData{
				PodCIDR: "10.244.1.0/24",
			},
			expectedResult: "Foo 10.244.1.0/24 Bar",
			expectedLog: `debug	successfully read template file	{"template_source": "/tmp/wireguard-controller-test-tpl.conf", "template_target": "/tmp/wireguard-controller-test.conf"}
debug	successfully parsed template file	{"template_source": "/tmp/wireguard-controller-test-tpl.conf", "template_target": "/tmp/wireguard-controller-test.conf"}
debug	successfully executed the template	{"template_source": "/tmp/wireguard-controller-test-tpl.conf", "template_target": "/tmp/wireguard-controller-test.conf"}
info	CNI config does not match desired config, will override it	{"template_source": "/tmp/wireguard-controller-test-tpl.conf", "template_target": "/tmp/wireguard-controller-test.conf"}
info	Successfully wrote CNI config	{"template_source": "/tmp/wireguard-controller-test-tpl.conf", "template_target": "/tmp/wireguard-controller-test.conf"}
`,
		},
		{
			name: "broken template",
			tpl:  "Foo {{ BROKEN_SHOULD_NOT_WORK }} Bar",
			data: tplData{
				PodCIDR: "10.244.1.0/24",
			},
			expectedLog: `debug	successfully read template file	{"template_source": "/tmp/wireguard-controller-test-tpl.conf", "template_target": "/tmp/wireguard-controller-test.conf"}
`,
			expectedErr: errors.New(`template: wireguard-controller-test-tpl.conf:1: function "BROKEN_SHOULD_NOT_WORK" not defined`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We cant use ioutil.TempFile() here because we need to have stable filenames to test the log output
			srcFile, err := os.OpenFile("/tmp/wireguard-controller-test-tpl.conf", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(srcFile.Name())
			if _, err := srcFile.Write([]byte(test.tpl)); err != nil {
				t.Fatal(err)
			}

			// We cant use ioutil.TempFile() here because we need to have stable filenames to test the log output
			targetFile, err := os.OpenFile("/tmp/wireguard-controller-test.conf", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(targetFile.Name())

			logOutput := &bytes.Buffer{}
			log := customlog.NewTestLog(zapcore.AddSync(logOutput))
			defer log.Sync()

			err = templateFile(log, srcFile.Name(), targetFile.Name(), test.data)
			if fmt.Sprint(err) != fmt.Sprint(test.expectedErr) {
				t.Error(err)
			}
			if test.expectedErr != nil {
				return
			}

			content, err := ioutil.ReadFile(targetFile.Name())
			if err != nil {
				t.Fatal(err)
			}

			testhelper.CompareStrings(t, test.expectedResult, string(content))

			// Test log output
			log.Sync()
			testhelper.CompareStrings(t, test.expectedLog, logOutput.String())
		})
	}
}
