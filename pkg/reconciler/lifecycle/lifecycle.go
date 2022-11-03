package lifecycle

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pivotal/kpack/pkg/config"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	coreinformers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	"github.com/pivotal/kpack/pkg/reconciler"
)

func NewController(
	ctx context.Context,
	configmapName string,
	configMapInformer coreinformers.ConfigMapInformer,
) *controller.Impl {
	key := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      configmapName,
	}

	wh := &Reconciler{
		key: key,
	}

	const queueName = "lifecycle"
	c := controller.NewContext(ctx, wh, controller.ControllerOptions{WorkQueueName: queueName, Logger: logging.FromContext(ctx).Named(queueName)})

	// Reconcile when the lifecycle configmap changes.
	configMapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(key.Namespace, key.Name),
		Handler:    controller.HandleAll(c.Enqueue),
	})

	return c
}

type Reconciler struct {
	key               types.NamespacedName
	ConfigMapLister   corelisters.ConfigMapLister
	Tracker           reconciler.Tracker
	K8sClient         k8sclient.Interface
	LifecycleProvider *config.LifecycleProvider
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, configMapName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("failed splitting meta namespace key: %s", err)
	}

	lifecycleConfigMap, err := c.ConfigMapLister.ConfigMaps(namespace).Get(configMapName)
	if err != nil {
		return err
	}

	return c.reconcileLifecycleImage(ctx, lifecycleConfigMap)
}

func (c *Reconciler) reconcileLifecycleImage(ctx context.Context, configMap *corev1.ConfigMap) error {
	/*
		get current state of c.lifecycleProvider.data (type: lifecycle)
		if current state == err: retry
		else:
		  if check current digest against digest in configmap: return
		  else: retry
	*/
	digest, err := c.LifecycleProvider.Digest()
	if err != nil {
		return err
	}
	/*
		Kind: ConfigMap
		Name: lifecycle-image
		Namespace: kpack
		Data:
		  image: "registry.io/foo"
	*/
	imageRef, ok := configMap.Data[config.LifecycleConfigKey]
	if !ok {
		return errors.Errorf("%s config invalid", config.LifecycleConfigName)
	}
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return err
	}
	// lifecycle digest has not changed
	if ref.Identifier() == digest {
		return nil
	}
	c.LifecycleProvider.UpdateImage()
	/*
		imageRef == "registry.io/foo
		OR
		imageRef == "registry.io/foo@sha256:abcdxyzetc
	*/
}
