package main

import (
	"context"
	"flag"
	api_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"time"
)

type PodController struct {
	//would it matter if I set this to kubernetes.ClientSet
	clientset kubernetes.Interface
	informer  cache.SharedIndexInformer
	queue     workqueue.RateLimitingInterface
}

type Event struct {
	key string
	pod *api_v1.Pod
}

func newPodController(clientset kubernetes.Interface, informer cache.SharedIndexInformer, queue workqueue.RateLimitingInterface) *PodController {
	return &PodController{
		clientset: clientset,
		informer:  informer,
		queue:     queue,
	}
}
func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	if err != nil {
		klog.Fatal(err)
	}

	//communicate with kube-apiserver
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		klog.Fatal(err)
	}

	//setup the ListWatcher to watch pods
	listwatch := &cache.ListWatch{
		ListFunc: func(options v1.ListOptions) (object runtime.Object, err error) {
			options.LabelSelector = "app:informers-demo"
			list, err := clientset.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), options)
			if err != nil {
				klog.Fatal(err)
				return nil, err
			}
			return list, nil

		},
		WatchFunc: func(options v1.ListOptions) (w watch.Interface, err error) {
			options.LabelSelector = "app:informers-demo"
			watch, err := clientset.CoreV1().Pods(v1.NamespaceAll).Watch(context.Background(), options)
			if err != nil {
				klog.Fatal(err)
				return nil, err
			}
			return watch, nil

		},
	}

	//setup the workqueue. RateLimiting allows us to re-add an event to the queue numTimes in case there was an error processing it the first time
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	//setup informer to be a shared informer that maintains an indexed cache. allows retrieving objs from cache using key
	//what is a resync period? why is it useful? is resync period even necessary?
	//difference between resync and relist?
	indexInformer := cache.NewSharedIndexInformer(listwatch, &api_v1.Pod{}, 30*time.Minute, cache.Indexers{})

	indexInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// is this the proper way to do this?
			key, err := cache.MetaNamespaceKeyFunc(obj)
			newEvent := Event{key: key, pod: (obj).(*api_v1.Pod).DeepCopy()}
			if err == nil {

				//what about type-casting to obj.(*api_v1.Pod).GetName()
				klog.Infof("pod created %s", key)
				queue.Add(newEvent)
			}

		},
		//called during resync as well
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			newEvent := Event{key: key, pod: (newObj).(*api_v1.Pod).DeepCopy()}
			//what about type-casting to obj.(*api_v1.Pod).GetName()
			if err == nil {
				klog.Infof("pod updated %s", key)
				queue.Add(newEvent)
			}

		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			newEvent := Event{key: key, pod: (obj).(*api_v1.Pod).DeepCopy()}
			if err == nil {

				//what about type-casting to obj.(*api_v1.Pod).GetName()
				klog.Infof("pod deleted %s", key)
				queue.Add(newEvent)
			}
		},
	})

}
