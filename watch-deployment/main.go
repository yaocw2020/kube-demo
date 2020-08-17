package main

import (
	"flag"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

func main() {
	// creates the in-cluster config
	// config, err := rest.InClusterConfig()
	// if err != nil {
	// klog.Fatalf("failed to get in cluster config, error: %v", err)
	//}
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal("failed to build kubernetes clientset: %v", err)
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(clientset, time.Minute)

	controller := NewController(clientset, kubeInformerFactory.Apps().V1().Deployments())
	stopCh := SetSignalHandler()
	kubeInformerFactory.Start(stopCh)
	if err := controller.Run(2, stopCh); err != nil {
		klog.Fatal("run controller failed, error: %+v", err)
	}
}

func SetSignalHandler() chan struct{} {
	stop := make(chan struct{})
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		close(stop)
	}()

	return stop
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
