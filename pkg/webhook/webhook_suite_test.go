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
	"fmt"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	admissionv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteName := "Webhook Integration Suite"
	RunSpecsWithDefaultAndCustomReporters(t, suiteName, []Reporter{printer.NewlineReporter{}, printer.NewProwReporter(suiteName)})
}

var testenv *envtest.Environment
var cfg *rest.Config
var clientset *kubernetes.Clientset

// clientTransport is used to force-close keep-alives in tests that check for leaks
var clientTransport *http.Transport

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testenv = &envtest.Environment{}
	// we're initializing webhook here and not in webhook.go to also test the envtest install code via WebhookOptions
	initializeWebhookInEnvironment()
	var err error
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())
	fmt.Printf("cfg = %+v\n", cfg)

	clientTransport = &http.Transport{}
	cfg.Transport = clientTransport

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	// TODO: check if this is needed
	// Prevent the metrics listener being created
	metrics.DefaultBindAddress = "0"

	close(done)
}, 60)

var _ = AfterSuite(func() {
	fmt.Println("stopping?")
	Expect(testenv.Stop()).To(Succeed())

	// Put the DefaultBindAddress back
	metrics.DefaultBindAddress = ":8080"
}, 60)

func initializeWebhookInEnvironment() {
	namespacedScopeV1Beta1 := admissionv1beta1.NamespacedScope
	namespacedScopeV1 := admissionv1.NamespacedScope
	failedTypeV1Beta1 := admissionv1beta1.Fail
	failedTypeV1 := admissionv1.Fail
	equivalentTypeV1Beta1 := admissionv1beta1.Equivalent
	equivalentTypeV1 := admissionv1.Equivalent
	noSideEffectsV1Beta1 := admissionv1beta1.SideEffectClassNone
	noSideEffectsV1 := admissionv1.SideEffectClassNone
	webhookPathV1 := "/failing"

	testenv.WebhookInstallOptions = envtest.WebhookInstallOptions{
		ValidatingWebhooks: []client.Object{
			&admissionv1beta1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deployment-validation-webhook-config",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "ValidatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1beta1",
				},
				Webhooks: []admissionv1beta1.ValidatingWebhook{
					{
						Name: "deployment-validation.kubebuilder.io",
						Rules: []admissionv1beta1.RuleWithOperations{
							{
								Operations: []admissionv1beta1.OperationType{"CREATE", "UPDATE"},
								Rule: admissionv1beta1.Rule{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
									Scope:       &namespacedScopeV1Beta1,
								},
							},
						},
						FailurePolicy: &failedTypeV1Beta1,
						MatchPolicy:   &equivalentTypeV1Beta1,
						SideEffects:   &noSideEffectsV1Beta1,
						ClientConfig: admissionv1beta1.WebhookClientConfig{
							Service: &admissionv1beta1.ServiceReference{
								Name:      "deployment-validation-service",
								Namespace: "default",
								Path:      &webhookPathV1,
							},
						},
					},
				},
			},
			&admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deployment-validation-webhook-config",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "ValidatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1beta1",
				},
				Webhooks: []admissionv1.ValidatingWebhook{
					{
						Name: "deployment-validation.kubebuilder.io",
						Rules: []admissionv1.RuleWithOperations{
							{
								Operations: []admissionv1.OperationType{"CREATE", "UPDATE"},
								Rule: admissionv1.Rule{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
									Scope:       &namespacedScopeV1,
								},
							},
						},
						FailurePolicy: &failedTypeV1,
						MatchPolicy:   &equivalentTypeV1,
						SideEffects:   &noSideEffectsV1,
						ClientConfig: admissionv1.WebhookClientConfig{
							Service: &admissionv1.ServiceReference{
								Name:      "deployment-validation-service",
								Namespace: "default",
								Path:      &webhookPathV1,
							},
						},
					},
				},
			},
		},
	}
}

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
