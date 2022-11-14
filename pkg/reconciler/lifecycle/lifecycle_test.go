package lifecycle_test

import (
	"github.com/pivotal/kpack/pkg/reconciler/lifecycle"
	"github.com/sclevine/spec"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"
	"testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/reconciler/image"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestLifecycleReconciler(t *testing.T) {
	spec.Run(t, "Lifecycle Reconciler", testLifecycleReconciler)
}

func testLifecycleReconciler(t *testing.T, when spec.G, it spec.S) {

	var (
		key         = types.NamespacedName{Namespace: "kpack", Name: "lifecycle-image"}
		fakeTracker = testhelpers.FakeTracker{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &lifecycle.Reconciler{
				Tracker:         fakeTracker,
				K8sClient:       k8sfakeClient,
				ConfigMapLister: listers.GetConfigMapLister(),
				LifecycleProvider:
			}

			return r, actionRecorderList, eventList
		})

	when("Reconcile", func() {
		it("updates observed generation after processing an update", func() {

			rt.Test(rtesting.TableRow{
				Key: key.String(),
				Objects: []runtime.Object{
					lifecycleConfigMap,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Image{
							ObjectMeta: image.ObjectMeta,
							Spec:       image.Spec,
							Status: buildapi.ImageStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: updatedGeneration,
									Conditions:         conditionReadyUnknown(),
								},
							},
						},
					},
				},
			})
		})

	})
}
