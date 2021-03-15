package webhook_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("Test", func() {

	Describe("Webhook", func() {
		BeforeEach(func() {
			Expect(cfg).NotTo(BeNil())
		})
		It("should reject create request for webhook that rejects all requests", func(done Done) {
			fmt.Printf("cfg test= %+v\n", cfg)
			m, err := manager.New(cfg, manager.Options{
				Port:    testenv.WebhookInstallOptions.LocalServingPort,
				Host:    testenv.WebhookInstallOptions.LocalServingHost,
				CertDir: testenv.WebhookInstallOptions.LocalServingCertDir,
			}) // we need manager here just to leverage manager.SetFields
			Expect(err).NotTo(HaveOccurred())
			server := m.GetWebhookServer()
			server.Register("/failing", &webhook.Admission{Handler: &rejectingValidator{}})

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				_ = server.Start(ctx)
			}()

			c, err := client.New(cfg, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			obj := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			}

			Eventually(func() bool {
				err = c.Create(context.TODO(), obj)
				return errors.ReasonForError(err) == metav1.StatusReason("Always denied")
			}, 1*time.Second).Should(BeTrue())

			cancel()
			close(done)
			fmt.Println("PASSED?")
		})

		It("should reject create request for unmanaged webhook that rejects all requests", func(done Done) {
			cluster, err := cluster.New(cfg, func(clusterOptions *cluster.Options) {})
			Expect(err).NotTo(HaveOccurred())

			opts := webhook.Options{
				Port:    testenv.WebhookInstallOptions.LocalServingPort,
				Host:    testenv.WebhookInstallOptions.LocalServingHost,
				CertDir: testenv.WebhookInstallOptions.LocalServingCertDir,
			}
			server, err := webhook.NewUnmanaged(cluster, opts)
			Expect(err).NotTo(HaveOccurred())
			server.Register("/failing", &webhook.Admission{Handler: &rejectingValidator{}})

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				_ = server.Start(ctx)
			}()

			c, err := client.New(cfg, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			obj := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			}

			Eventually(func() bool {
				err = c.Create(context.TODO(), obj)
				return errors.ReasonForError(err) == metav1.StatusReason("Always denied")
			}, 1*time.Second).Should(BeTrue())

			cancel()
			close(done)
		})
	})
})

type rejectingValidator struct {
}

func (v *rejectingValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	return admission.Denied(fmt.Sprint("Always denied"))
}

//import (
//	"context"
//
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//	"sigs.k8s.io/controller-runtime/pkg/manager"
//	. "sigs.k8s.io/controller-runtime/pkg/webhook"
//	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
//)
//
//var _ = Describe("webhook", func() {
//
//	BeforeEach(func() {
//		Expect(cfg).NotTo(BeNil())
//	})
//
//	Describe("webhook", func() {
//		It("should serve webhook requests", func(done Done) {
//			By("Creating the Manager")
//			mgr, err := manager.New(cfg, manager.Options{})
//			Expect(err).NotTo(HaveOccurred())
//
//			By("Creating the webhook server")
//			hookServer := &Server{
//				Port: 8443,
//			}
//
//			By("Building the webhooks")
//			mutatingHook := &Admission{
//				Handler: admission.HandlerFunc(func(ctx context.Context, req AdmissionRequest) AdmissionResponse {
//					return Patched("some changes",
//						JSONPatchOp{Operation: "add", Path: "/metadata/annotations/access", Value: "granted"},
//						JSONPatchOp{Operation: "add", Path: "/metadata/annotations/reason", Value: "not so secret"},
//					)
//				}),
//			}
//
//			validatingHook := &Admission{
//				Handler: admission.HandlerFunc(func(ctx context.Context, req AdmissionRequest) AdmissionResponse {
//					return Denied("none shall pass!")
//				}),
//			}
//
//			By("registering the webhooks in the server")
//			hookServer.Register("/mutating", mutatingHook)
//			hookServer.Register("/validating", validatingHook)
//
//			By("adding it to the manager")
//			if err := mgr.Add(hookServer); err != nil {
//				panic(err)
//			}
//
//			By("Starting the Manager")
//			ctx, cancel := context.WithCancel(context.Background())
//			defer cancel()
//			go func() {
//				defer GinkgoRecover()
//				Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
//			}()
//
//			close(done)
//		}, 5)
//	})
//})
