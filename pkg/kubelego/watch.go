package kubelego

import (
	"reflect"
	"time"

	"github.com/jetstack/kube-lego/pkg/ingress"

	k8sMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	k8sExtensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func ingressListFunc(c *kubernetes.Clientset, ns string) func(k8sMeta.ListOptions) (runtime.Object, error) {
	return func(opts k8sMeta.ListOptions) (runtime.Object, error) {
		return c.Extensions().Ingresses(ns).List(opts)
	}
}

func ingressWatchFunc(c *kubernetes.Clientset, ns string) func(options k8sMeta.ListOptions) (watch.Interface, error) {
	return func(options k8sMeta.ListOptions) (watch.Interface, error) {
		return c.Extensions().Ingresses(ns).Watch(options)
	}
}

// requestReconfigure will trigger a resync of *all* ingress resources.
func (kl *KubeLego) requestReconfigure() error {
	allIng, err := ingress.All(kl)
	if err != nil {
		return err
	}
	for _, ing := range allIng {
		key, err := cache.MetaNamespaceKeyFunc(ing.Object)
		if err != nil {
			return err
		}
		kl.workQueue.AddRateLimited(key)
	}
	return nil
}

func (kl *KubeLego) WatchReconfigure() {

	kl.workQueue = workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(time.Minute*10, time.Hour*24), "kube-lego")

	// handle worker shutdown
	go func() {
		<-kl.stopCh
		kl.workQueue.ShutDown()
	}()

	go func() {
		kl.waitGroup.Add(1)
		defer kl.waitGroup.Done()
		for {
			item, quit := kl.workQueue.Get()
			if quit {
				return
			}
			func(item interface{}) {
				defer kl.workQueue.Done(item)
				key, ok := item.(string)
				if !ok {
					kl.Log().Errorf("worker: invalid item in workqueue: %v", item)
					kl.workQueue.Forget(item)
					return
				}
				name, namespace, err := cache.SplitMetaNamespaceKey(key)
				if err != nil {
					kl.Log().Errorf("worker: invalid string in workqueue: %s", item)
					kl.workQueue.Forget(item)
					return
				}
				kl.Log().Debugf("worker: begin processing %v", key)
				ing := ingress.New(kl, namespace, name)
				if ing.Exists == false {
					kl.Log().Errorf("worker: ingress for key %q no longer exists. Skipping...", key)
					kl.workQueue.Forget(item)
					return
				}
				err = kl.reconfigure(ing)
				if err != nil {
					kl.Log().Errorf("worker: error processing item: %v", err)
					return
				}
				kl.Log().Debugf("worker: done processing %v", key)
				kl.workQueue.Forget(item)
			}(item)
		}
	}()
}

func (kl *KubeLego) WatchEvents() {

	kl.Log().Debugf("start watching ingress objects")

	resyncPeriod := 10 * time.Minute

	ingEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*k8sExtensions.Ingress)
			if ingress.IgnoreIngress(addIng) != nil {
				return
			}
			kl.Log().Debugf("CREATE ingress/%s/%s", addIng.Namespace, addIng.Name)
			if key, err := cache.MetaNamespaceKeyFunc(addIng); err != nil {
				kl.Log().Errorf("worker: failed to key ingress: %v", err)
				return
			} else {
				kl.workQueue.AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			delIng := obj.(*k8sExtensions.Ingress)
			if ingress.IgnoreIngress(delIng) != nil {
				return
			}
			kl.Log().Debugf("DELETE ingress/%s/%s", delIng.Namespace, delIng.Name)
			if key, err := cache.MetaNamespaceKeyFunc(delIng); err != nil {
				kl.Log().Errorf("worker: failed to key ingress: %v", err)
				return
			} else {
				kl.workQueue.AddRateLimited(key)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			oldIng := old.(*k8sExtensions.Ingress)
			upIng := cur.(*k8sExtensions.Ingress)

			//ignore resource version in equality check
			oldIng.ResourceVersion = ""
			upIng.ResourceVersion = ""

			if !reflect.DeepEqual(oldIng, upIng) {
				upIng := cur.(*k8sExtensions.Ingress)
				if ingress.IgnoreIngress(upIng) != nil {
					return
				}
				kl.Log().Debugf("UPDATE ingress/%s/%s", upIng.Namespace, upIng.Name)
				if key, err := cache.MetaNamespaceKeyFunc(upIng); err != nil {
					kl.Log().Errorf("worker: failed to key ingress: %v", err)
					return
				} else {
					kl.workQueue.AddRateLimited(key)
				}
			}
		},
	}

	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(kl.kubeClient, kl.legoWatchNamespace),
			WatchFunc: ingressWatchFunc(kl.kubeClient, kl.legoWatchNamespace),
		},
		&k8sExtensions.Ingress{},
		resyncPeriod,
		ingEventHandler,
	)

	go controller.Run(kl.stopCh)
}
