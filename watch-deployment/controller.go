package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

const (
	// actions in event
	Add = "add"
	Update = "update"
	Delete = "delete"
)

// Controller is the controller for deployment
type Controller struct {
	kubeclientset kubernetes.Interface
	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	workqueue workqueue.RateLimitingInterface
}

type event struct {
	action string
	key string
}

// NewController return a controller
func NewController(kubeclientset kubernetes.Interface, deploymentInformer appsinformers.DeploymentInformer) *Controller {
	controller := &Controller{
		kubeclientset: kubeclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "demo-deployment"),
	}

	klog.Info("Setting up event handlers")

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.enqueue(Add, obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueue(Update, new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueue(Delete, obj)
		},
	})

	return controller
}


// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting demo controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {}
}

func (c *Controller) processNextWorkItem() bool {
	item, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// wrap in a func so we can defer c.workqueue.Done
	func(item interface{}) {
		defer c.workqueue.Done(item)
		defer c.workqueue.Forget(item)
		e, ok := item.(event)
		if !ok {
			klog.Errorf("invalid event from workqueue, %+v\n", item)
			return
		}

		namespace, name, err := cache.SplitMetaNamespaceKey(e.key)
		if err != nil {
			klog.Errorf("get namespace and name failed, error: %+v, key: %s\n", err, e.key)
			return
		}

		if e.action == Delete {
			klog.Infof("delete deployment, namespace: %s, name: %s\n", namespace, name)
			return
		}

		obj, err := c.kubeclientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("get deployment failed, error: %+v, namespace: %s, name: %s\n", err, namespace, name)
			return
		}
		klog.Infof("%s deployment, obj: %+v\n", e.action, obj)
	}(item)

	return true
}

func (c *Controller) enqueue(action string, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.Add(event{action, key})
}

