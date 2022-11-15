package lifecycle_test

import (
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/pivotal/kpack/pkg/config"
	"github.com/pivotal/kpack/pkg/reconciler/lifecycle"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"
	"testing"

	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestLifecycleReconciler(t *testing.T) {
	spec.Run(t, "Lifecycle Reconciler", testLifecycleReconciler)
}

func testLifecycleReconciler(t *testing.T, when spec.G, it spec.S) {

	var (
		fakeTracker        = testhelpers.FakeTracker{}
		lifecycleImage     = randomImage(t)
		lifecycleImageRef  = "gcr.io/lifecycle@sha256:some-sha"
		serviceAccountName = "lifecycle-sa"
		namespace          = "kpack"
		key                = types.NamespacedName{Namespace: namespace, Name: config.LifecycleConfigName}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			imageFetcher := registryfakes.NewFakeClient()
			fakeKeychain := &registryfakes.FakeKeychain{}
			imageFetcher.AddImage(lifecycleImageRef, lifecycleImage, fakeKeychain)
			fakeKeychainFactory := &registryfakes.FakeKeychainFactory{}
			secretRef := registry.SecretRef{ServiceAccount: serviceAccountName, Namespace: namespace}
			fakeKeychainFactory.AddKeychainForSecretRef(t, secretRef, fakeKeychain)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &lifecycle.Reconciler{
				Tracker:           fakeTracker,
				K8sClient:         k8sfakeClient,
				ConfigMapLister:   listers.GetConfigMapLister(),
				LifecycleProvider: config.NewLifecycleProvider(imageFetcher, fakeKeychainFactory),
			}

			return r, actionRecorderList, eventList
		})

	when("Reconcile", func() {
		it("can load lifecycle image", func() {

			lifecycleConfigMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.LifecycleConfigName,
					Namespace: namespace,
				},
				Data: map[string]string{
					config.LifecycleConfigKey:,
				},
			}

			rt.Test(rtesting.TableRow{
				Key: key.String(),
				Objects: []runtime.Object{
					lifecycleConfigMap,
				},
				WantErr: false,
				//WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				//	{
				//		Object: &buildapi.Image{
				//			ObjectMeta: image.ObjectMeta,
				//			Spec:       image.Spec,
				//			Status: buildapi.ImageStatus{
				//				Status: corev1alpha1.Status{
				//					ObservedGeneration: updatedGeneration,
				//					Conditions:         conditionReadyUnknown(),
				//				},
				//			},
				//		},
				//	},
				//},
			})
		})

	})
}

func randomImage(t *testing.T) ggcrv1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}
