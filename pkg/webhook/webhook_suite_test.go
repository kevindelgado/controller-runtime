/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteName := "Webhook Integration Suite"
	RunSpecsWithDefaultAndCustomReporters(t, suiteName, []Reporter{printer.NewlineReporter{}, printer.NewProwReporter(suiteName)})
}

//var testenv *envtest.Environment
//var cfg *rest.Config
//var clientset *kubernetes.Clientset
//
//// clientTransport is used to force-close keep-alives in tests that check for leaks
//var clientTransport *http.Transport
//
//var _ = BeforeSuite(func(done Done) {
//	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
//
//	err := (&crscheme.Builder{
//		GroupVersion: schema.GroupVersion{Group: "chaosapps.metamagical.io", Version: "v1"},
//	}).
//		Register(
//			// TODO: dont use controllertest types
//			&controllertest.UnconventionalListType{},
//			&controllertest.UnconventionalListTypeList{},
//		).AddToScheme(scheme.Scheme)
//	Expect(err).To(BeNil())
//
//	testenv = &envtest.Environment{
//		CRDDirectoryPaths: []string{"testdata/crds"},
//	}
//
//	cfg, err = testenv.Start()
//	Expect(err).NotTo(HaveOccurred())
//
//	clientTransport = &http.Transport{}
//	cfg.Transport = clientTransport
//
//	clientset, err = kubernetes.NewForConfig(cfg)
//	Expect(err).NotTo(HaveOccurred())
//
//	// Prevent the metrics listener being created
//	metrics.DefaultBindAddress = "0"
//
//	close(done)
//}, 60)
//
//var _ = AfterSuite(func() {
//	Expect(testenv.Stop()).To(Succeed())
//
//	// Put the DefaultBindAddress back
//	metrics.DefaultBindAddress = ":8080"
//})
//
//var _ = BeforeSuite(func(done Done) {
//	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
//
//	close(done)
//}, 60)
