package cniconfig

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	testhelper "github.com/mrincompetent/wireguard-controller/pkg/test"
	"go.uber.org/zap/zaptest"
)

func TestTemplateFile(t *testing.T) {
	tests := []struct {
		name           string
		data           tplData
		tpl            string
		expectedResult string
		expectedErr    error
	}{
		{
			name: "simple template",
			tpl:  "Foo {{ .PodCIDR }} Bar",
			data: tplData{
				PodCIDR: "10.244.1.0/24",
			},
			expectedResult: "Foo 10.244.1.0/24 Bar",
		},
		{
			name: "broken template",
			tpl:  "Foo {{ BROKEN_SHOULD_NOT_WORK }} Bar",
			data: tplData{
				PodCIDR: "10.244.1.0/24",
			},
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

			err = templateFile(zaptest.NewLogger(t), srcFile.Name(), targetFile.Name(), test.data)
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
		})
	}
}
