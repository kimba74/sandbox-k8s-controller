package examples

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	appType "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appInformer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appLister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// DeploymentController is the controller implementation
type DeploymentController struct {
	clientset   kubernetes.Interface
	informer    cache.InformerSynced
	lister      appLister.DeploymentLister
	workerqueue workqueue.RateLimitingInterface
}

// NewDeploymentController create s new instance of the DeploymentController.
func NewDeploymentController(clientset kubernetes.Interface, informer appInformer.DeploymentInformer) *DeploymentController {
	// NOTE: When this is an implementation of a Custom Reource Controller the
	//       custom scheme will have to be added to the K8s scheme in order to
	//       log events for the custom resource controller
	// NOTE: Do I need an Event Broadcaster??
	//       If so create it here:
	//          eventBroadcaster := record.NewBroadcaster()
	//          eventBroadcaster.StartLogging(glog.Infof)
	//          eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	//          recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Create controller struct
	controller := &DeploymentController{
		clientset:   clientset,
		informer:    informer.Informer().HasSynced,
		lister:      informer.Lister(),
		workerqueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	// Add the controller's logic as event handler for deployment events
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		DeleteFunc: controller.handleObject,
		// Only handle events if the resource version has changed otherwise it
		// is a periodic refresh of the resource
		UpdateFunc: func(old, new interface{}) {
			oldDep := old.(*appType.Deployment)
			newDep := new.(*appType.Deployment)
			if oldDep.ResourceVersion != newDep.ResourceVersion {
				controller.handleObject(new)
			}
		},
	})

	// Return the created controller
	return controller
}

// Run initializes the DeploymentController's workers and starts them until
// the stopChan is being closed.
func (c *DeploymentController) Run(parallelism int, stopChan <-chan struct{}) error {
	// Defer the crash handling and closing of the workerqueue to after this method exits
	defer runtime.HandleCrash()
	defer c.workerqueue.ShutDown()

	glog.Info("Staring DeploymentController")

	// First we wait for the informer to sync, then we can start the workers.
	glog.Info("Waiting for Informer to sync")
	if ok := cache.WaitForCacheSync(stopChan, c.informer); !ok {
		// If something goes wrong while we wait for the informer(s) to sync we will
		// exit the method and return an error.
		return fmt.Errorf("failed to wait for cache to sync")
	}

	// Now we go over the parallelism passed to the method and start as many workers
	// as were requested. The workers are started as background go routines.
	glog.Info("Starting workers")
	for i := 0; i < parallelism; i++ {
		go wait.Until(c.runWorker, time.Second, stopChan)
		glog.Infof("  * Worker %d started", i+1)
	}

	// After the workers were started the method will hold until the program was
	// terminated and the channel was closed.
	glog.Info("Started workers")
	<-stopChan
	glog.Info("Stopping workers")

	// Everything went well and we can return from the method without any errors.
	return nil
}

// Handle all received objects, check them if they are valid, were deleted or updated.
func (c *DeploymentController) handleObject(obj interface{}) {
	var object metaV1.Object
	var ok bool
	// Check if the object received is a K8s resource object
	if object, ok = obj.(metaV1.Object); !ok {
		// The received object is not a K8s resource object. Now check if the
		// actual resource object was deleted but the watch deletion event was missed
		// in this case a deletion placeholder will be inserted into a DeltaFIFO queue.
		tombestone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			// The received object was not a deletion placeholder for the K8s
			// resource object. At this point we have no idea what the received is.
			// The error will be logged and the method will return.
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		// The received object was a deletion placeholder for a deleted K8s resource
		// object. Now we try to retrieve the original resource object from the
		// placeholder and check if it is a valid K8s resource object.
		object, ok = tombestone.Obj.(metaV1.Object)
		if !ok {
			// The encapsulated object inside the deletion placeholder was not a
			// valid K8s resource object. Nothing further can be done at this point
			// to recover the deleted object, we therefore will log an error and
			// have the method return.
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}
	// NOTE: Here we could check for the owner of the K8s resource object if our
	// custom resource definition is meant to be a "controller" of another
	// K8s resource. (e.g. Deployment -> Replication Controller -> Pod)
	glog.Infof("HandleObject: Resource (%s/%s ver. %s) received for handling",
		object.GetNamespace(),
		object.GetName(),
		object.GetResourceVersion())

	// For later processing we create the NamespaceKey for the K8s resource object
	// and add it to the worker queue. The reason for adding the NamespaceKey instead
	// of the K8s resource object itself is space and the fact that the underlying
	// object might have been changed again by the time a worker gets to it.
	var key string
	var err error
	// Try creating the NamespaceKey (namespace/name) for the K8s resource object
	if key, err = cache.MetaNamespaceKeyFunc(object); err != nil {
		// The NamespaceKey could not be created so we let log an error and return
		runtime.HandleError(err)
		return
	}
	// Now we add the NamespaceKey to the worker queue for the workers to pick up.
	c.workerqueue.AddRateLimited(key)
	glog.Infof("HandleObject: ResourceKey (%s) generated for resourced and enqueued", key)
}

// runWorker represents an actual queue worker. All this method does is run a while
// loop over the worker queue and process the items that were placed in it.
// The reason for this simple logic being placed in its own method is to allow it
// to be run as a go routine multiple times for concurrent processing of the queue.
func (c *DeploymentController) runWorker() {
	// Run a "while" loop over the worker queue and process the items inside the
	// queue. This method will exit once the processNect() method returns false.
	for c.processNext() {
	}
	glog.Info("Worker stopped processing")
}

// processNext
func (c *DeploymentController) processNext() bool {
	// Get the next item from the worker queue. If the queue is empty this call
	// will wait until a new item is being placed in the queue. In case the queue
	// is being closed the Get() method will return 'true' as the secind return
	// parameter.
	object, shutdown := c.workerqueue.Get()

	// The worker queue was closed and the Get() method returned 'true' as the
	// second return parameter. In this case the method will return 'false' causing
	// the worker (processNext()) logic to end and exit gracefully.
	if shutdown {
		glog.Info("Shutting down worker")
		return false
	}

	glog.Infof("ProcessNext: Object (%v) read from workerqueue", object)

	// The actual processing of the object retrieved from the queue will be wrapped
	// into a function so we can defer the workerqueue.Done() method call.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// don't want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workerqueue.Done(obj)

		// First we are trying to retrieve the NamespaceKey from the queue and
		// check if it is actualy of type 'string' before unmarshalling it back
		// into 'namespace' and object 'name'.
		var namespaceKey string
		var ok bool
		if namespaceKey, ok = obj.(string); !ok {
			// The object in the worker queue is not an expected string and therefore
			// can't be a NamespaceKey. We will take the object from the queue to
			// prevent it from being processed over and over again. Then we log an
			// error and just skip to the next item. Since it was not a valid object
			// we won't return an error but exit out of the function.
			c.workerqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected 'string' in the worker queue but instead got %#v", obj))
			return nil
		}

		glog.Infof("ProcessNext: Object (%s) is of type string, try syncing", namespaceKey)

		// With the object retrieved from the queue verified to be of type 'string'
		// we now try to do some actual work on the underlying K8s resource obeject.
		// The method 'syncResource()' will try to parse the namespaceKey and do
		// its work. If it can't process the K8s resource object it will return an
		// error, which we will re-throw and exit the function.
		// NOTE: This block will log the error being thrown by the syncResource()
		// method but will NOT forget the item, that way it will be left in the queue
		// and attempted to be processed again. The deferred workerqueue.Done() will
		// mark this item as finished but the missing workerqueue.Forget() ensures
		// the object will remain in the worker queue.
		if err := c.syncResource(namespaceKey); err != nil {
			return fmt.Errorf("Error syncing resource %s: %s", namespaceKey, err.Error())
		}

		glog.Infof("ProcessNext: Resource (%s) was synced successfully", namespaceKey)

		// With the processing work done we will remove the object from the worker
		// queue to prevent it from being processed over and over again...
		c.workerqueue.Forget(obj)
		glog.Infof("ProcessNext: Resource (%s) marked as finished in the rateLimiter", namespaceKey)
		//...then we leave function gracefully.
		return nil
	}(object)

	glog.Infof("ProcessNext: Resource (%s) Done processing", object)

	// In case the processing logic experienced a problem and returned an error
	// we will log the error but return 'true' so the worker won't get stopped.
	if err != nil {
		runtime.HandleError(err)
	}

	// Returning 'true' so the worker won't get stopped and continues processing
	// incoming objects.
	return true
}

// syncResource
func (c *DeploymentController) syncResource(namespaceKey string) error {
	glog.Infof("SyncResource: String (%s) check if NamespaceKey and try splitting it", namespaceKey)
	// Split the NamespaceKey back into 'namespace' and 'name'. If something goes
	// wrong we handle the resulting error by logging it and returning from the
	// method without returning an error.
	namespace, name, err := cache.SplitMetaNamespaceKey(namespaceKey)
	if err != nil {
		runtime.HandleError(fmt.Errorf("string %s could not be split into 'namespace' and resource 'name'", namespaceKey))
		return nil
	}

	glog.Infof("SyncResource: NamespaceKey (%s) successfully split back into namespace (%s) and resource (%s)", namespaceKey, namespace, name)

	// With the 'namespace' and the 'name' we'll try to retrieve the K8s resource
	// object from the corresponding lister. If the resource object cannot be
	// retrieved from the lister an error will be returned.
	dc, err := c.lister.Deployments(namespace).Get(name)

	// In case an error is being returned we will take a look at the reason.
	if err != nil {
		// If the error was returned because the object wasn't found we will log
		// an error and return from the 'syncResource' method gracefully without
		// passing on the error.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("resource %s was not found, returning", namespaceKey))
			return nil
		}
		// If the error was anything else besides the resource object not being found
		// we will exit the method by passing on the error for further error handling.
		return err
	}

	glog.Infof("SyncResource: Resource (%s) successfully loaded from lister", namespaceKey)

	// NOTE: Here we do something with the Deployment resource. For simplicity we
	// will just add a 'controller-check' label to demonstrate that our controller
	// has actually touched the resource.

	// First, let's get the labels from the Deployment resource object we successfully
	// retrieved before.
	labels := dc.GetLabels()
	// Now lets check if the 'controller-chack' label already exists. If it doesn't
	// we create a new one and add a message.
	glog.Infof("SyncResource: Resource (%s) check if label 'controller-ckeck' already exists", namespaceKey)
	if _, ok := labels["constroller-check"]; !ok {
		glog.Infof("SyncResource: Resource (%s) does not have label 'constroller-check', yet. Creating it now!", namespaceKey)
		labels["constroller-check"] = "done"
		dc.SetLabels(labels)
		// Here we update the actual Deployment resource object using the Kubernetes
		// client-set. If the update process returns an error we will exit the
		// 'syncResource' method by passing on the encountered error for later
		// error handling.
		dc, err = c.clientset.AppsV1().Deployments(namespace).Update(dc)
		if err != nil {
			return err
		}
		glog.Infof("Resource (%s) label set", namespaceKey)
	}
	// The update of the Deployment resource object was successful and we will
	// exit the 'syncResource' method gracefully.
	return nil
}
