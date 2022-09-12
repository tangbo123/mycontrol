package pkg

import (
	"context"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	v14 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v13 "k8s.io/client-go/informers/core/v1"
	v1betai1 "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/listers/core/v1"

	v1beta12 "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	"time"
)

type controller struct {
	client        kubernetes.Interface
	stopch        <-chan struct{}
	ingressLister v1beta12.IngressLister
	serviceLister v12.ServiceLister
	queue         workqueue.RateLimitingInterface
}

func (c *controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	c.queue.Add(key)
}

func (c *controller) addService(obj interface{}) {
	c.enqueue(obj)
}

func (c *controller) UpdateService(obj interface{}, obj2 interface{}) {
	if reflect.DeepEqual(obj, obj2) {
		return
	}

	c.enqueue(obj2)
}

func (c *controller) Run(stopCh chan struct{}) {
	for i := 0; i < 5; i++ {
		//保证始终有5个
		go wait.Until(c.worker, time.Minute, stopCh)
	}

	<-stopCh
}

func (c *controller) worker() {
	for c.processNextItem() {

	}
}

func (c *controller) processNextItem() bool {
	item, shoutdown := c.queue.Get()
	if shoutdown {
		return false
	}
	defer c.queue.Done(item)

	key := item.(string)
	err := c.syncService(key)
	if err != nil {
		c.handleError(key, err)
	}

	return true
}

func (c *controller) syncService(key string) error {
	namespaceKey, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	//service 不存在忽略
	service, err := c.serviceLister.Services(namespaceKey).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	_, ok := service.GetAnnotations()["ingress/http"]
	ingress, err := c.ingressLister.Ingresses(namespaceKey).Get(name)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if ok && errors.IsNotFound(err) {
		//service 存在 ingress 不存在
		ig := c.constructIngress(namespaceKey, name)
		_, err = c.client.ExtensionsV1beta1().Ingresses(namespaceKey).Create(
			context.TODO(),
			ig,
			v14.CreateOptions{},
		)
		if err != nil {
			return err
		}
	} else if !ok && ingress != nil {
		//删除ingress
		err := c.client.ExtensionsV1beta1().Ingresses(namespaceKey).Delete(context.TODO(), name, v14.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil

}

func (c *controller) constructIngress(Namespace string, name string) *v1beta1.Ingress {
	ingress := v1beta1.Ingress{}
	ingress.APIVersion = "extensions/v1beta1"
	ingress.Name = name
	ingress.Namespace = Namespace
	pathType := v1beta1.PathTypePrefix
	ingress.Spec = v1beta1.IngressSpec{
		Rules: []v1beta1.IngressRule{
			{
				Host: "example.com",
				IngressRuleValue: v1beta1.IngressRuleValue{
					HTTP: &v1beta1.HTTPIngressRuleValue{
						Paths: []v1beta1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: v1beta1.IngressBackend{
									ServiceName: name,
									ServicePort: intstr.IntOrString{IntVal: 80},
								},
							},
						},
					},
				},
			},
		},
	}
	return &ingress
}

func (c *controller) deleteIngress(obj interface{}) {
	ingress := obj.(*v1beta1.Ingress)
	OwnerReference := v14.GetControllerOf(ingress)
	if OwnerReference == nil || OwnerReference.Kind != "Service" {
		return
	}

	if OwnerReference.Kind != "Service" {
		return
	}

	c.queue.Add(ingress.Namespace + "/" + ingress.Name)

}

func (c *controller) handleError(key string, err error) {
	if c.queue.NumRequeues(key) <= 10 {
		c.queue.AddRateLimited(key)
		return
	}

	runtime.HandleError(err)
	c.queue.Forget(key)
}

func NewController(client kubernetes.Interface, serviceInformer v13.ServiceInformer, ingressInformer v1betai1.IngressInformer, stopch <-chan struct{}) controller {

	c := controller{
		client:        client,
		stopch:        stopch,
		ingressLister: ingressInformer.Lister(),
		serviceLister: serviceInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressManger"),
	}
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addService,
		UpdateFunc: c.UpdateService,
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteIngress,
	})
	return c
}
