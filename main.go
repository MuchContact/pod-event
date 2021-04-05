/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"encoding/json"
	"events.com/pod/mysql/service"
	mysql_common "events.com/pod/mysql/tool"
	"flag"
	"fmt"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"net/url"
	"path/filepath"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
}

// NewController creates a new Controller.
func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller) *Controller {
	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	//err := c.syncToStdout(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	//c.handleErr(err, key)
	return true
}

// syncToStdout is the business logic of the controller. In this controller it simply prints
// information about the pod to stdout. In case an error happened, it has to simply return the error.
// The retry logic should not be part of the business logic.
func (c *Controller) syncToStdout(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		// Below we will warm up our cache with a Pod, so that we will see a delete for one pod
		fmt.Printf("Pod %s does not exist anymore\n", key)
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a Pod was recreated with the same name
		fmt.Printf("Sync/Add/Update for Pod %s\n", obj.(*v1.Pod).GetName())
	}
	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	klog.Infof("Dropping pod %q out of the queue: %v", key, err)
}

// Run begins watching and syncing.
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	klog.Info("Starting Pod controller")

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping Pod controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

var (
	argSinks string
)

func main() {
	//"mysql:?root:password@tcp(192.168.50.12:31112)/events?charset=utf8"
	flag.StringVar(&argSinks, "sink", "", "external sink(s) that receive events")
	flag.Parse()

	//u, err := url.Parse("http://bing.com/search?q=dotnet")
	uri, err3 := url.Parse(argSinks)
	if err3 != nil {
		klog.Fatal("", err3)
	}
	mysqlSvc, dbErr := service.CreateMysqlSink(uri)
	if dbErr != nil {
		klog.Fatal("", dbErr)
	}
	defer mysqlSvc.Stop()

	var kubeconfig string
	var master string
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// create the pod watcher
	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", v1.NamespaceDefault, fields.Everything())

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the pod key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Pod than the version which was responsible for triggering the update.
	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			switch obj.(type) {
			case *v1.Pod:
				pod := obj.(*v1.Pod)
				marshal, _ := json.Marshal(pod.Status)
				fmt.Printf("[A]name: %s, status: %s \n", pod.Name, marshal)
				point := &mysql_common.MysqlKubeEventPoint{
					Name:        pod.Name,
					Namespace:   pod.Namespace,
					Kind:        "CREATE",
					Type:        string(pod.Status.Phase),
					Reason:      "",
					Message:     string(marshal),
					TriggerTime: time.Now().Format("2006-01-02 15:04:05"),
				}
				mysqlSvc.ExportEvents(point)
			}
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			switch new.(type) {
			case *v1.Pod:
				pod := new.(*v1.Pod)
				marshal, _ := json.Marshal(pod.Status)
				fmt.Printf("[U]name: %s, status: %s \n", pod.Name, marshal)
				point := &mysql_common.MysqlKubeEventPoint{
					Name:        pod.Name,
					Namespace:   pod.Namespace,
					Kind:        "UPDATE",
					Type:        string(pod.Status.Phase),
					Reason:      "",
					Message:     string(marshal),
					TriggerTime: time.Now().Format("2006-01-02 15:04:05"),
				}
				mysqlSvc.ExportEvents(point)
			default:

			}

			//if pod != nil && len(pod.Status.ContainerStatuses)>0 &&  pod.Status.ContainerStatuses[0].LastTerminationState.Terminated !=nil {
			//	fmt.Println("Pod Event: ", pod.Namespace, pod.Name, pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Reason)
			//}
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			switch obj.(type) {
			case *v1.Pod:
				pod := obj.(*v1.Pod)
				marshal, _ := json.Marshal(pod.Status)
				fmt.Printf("[D]name: %s, status: %s \n", pod.Name, marshal)
				point := &mysql_common.MysqlKubeEventPoint{
					Name:        pod.Name,
					Namespace:   pod.Namespace,
					Kind:        "DELETE",
					Type:        string(pod.Status.Phase),
					Reason:      "",
					Message:     string(marshal),
					TriggerTime: time.Now().Format("2006-01-02 15:04:05"),
				}
				mysqlSvc.ExportEvents(point)
			default:
				fmt.Printf("[D] %s \n", reflect.TypeOf(obj))
			}

			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	controller := NewController(queue, indexer, informer)

	// We can now warm up the cache for initial synchronization.
	// Let's suppose that we knew about a pod "mypod" on our last run, therefore add it to the cache.
	// If this pod is not there anymore, the controller will be notified about the removal after the
	// cache has synchronized.
	indexer.Add(&v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "mypod",
			Namespace: v1.NamespaceDefault,
		},
	})

	// Now let's start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	// Wait forever
	select {}
}
