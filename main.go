package main

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"mycontro/pkg"
)

func main() {

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)

	if err != nil {
		inCLusterConfig, err := rest.InClusterConfig()
		config = inCLusterConfig

		if err != nil {
			log.Fatalln("无法获取config")
		}
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln("无法获取client")
	}

	factory := informers.NewSharedInformerFactory(clientSet, 0)
	ServiceInformer := factory.Core().V1().Services()
	ingressesInformer := factory.Extensions().V1beta1().Ingresses()

	//factory.Networking().V1().Ingresses()
	stopCh := make(chan struct{})

	control := pkg.NewController(clientSet, ServiceInformer, ingressesInformer, stopCh)

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	control.Run(stopCh)
}
