package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kimba74/sandbox-k8s-controller/examples"
	"k8s.io/apimachinery/pkg/runtime"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	flag.Parse()

	//===========================================================================
	// Initializing connection to the cluster

	// Load the configuration from the kube client config file
	config, err := clientcmd.BuildConfigFromFlags("", "/home/steffen/.kube/config")
	if err != nil {
		log.Fatalf("Error loading config: %s\n", err)
	}

	// Create and connect a k8s client
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating k8s client: %s\n", err)
	}

	//===========================================================================
	// Listing, creating, and deleting resources (CRUD example)

	nsExample := examples.NewNSClient("goclient-test", k8sClient)

	nsExample.RunCRUDExampe()

	//===========================================================================
	// Inspecting the Scheme
	rtScheme := runtime.NewSchemeBuilder(myTest) //NewScheme()
	fmt.Println("New Runtime Scheme:")
	for k := range rtScheme {
		fmt.Printf("   * Func: %v\n", k)
	}

	k8sScheme := scheme.Scheme
	fmt.Println("Kubernetes Scheme:")
	for k := range k8sScheme.AllKnownTypes() {
		fmt.Printf("   * Type: %s [%s]\n", k.Kind, k.Version)
	}

	//===========================================================================
	// Listening to resource changes

	// Get informer factory for kubernetes client-set
	k8sInfFactory := informers.NewSharedInformerFactory(k8sClient, time.Second*30)

	// Setup a channel to listen for program termination
	stopCh := setupSignls()

	// Starting factory so it starts processing events
	go k8sInfFactory.Start(stopCh)

	controller := examples.NewDeploymentController(k8sClient, k8sInfFactory.Apps().V1().Deployments())
	controller.Run(2, stopCh)

	<-stopCh
	fmt.Println("Ending program execution")
}

func myTest(s *runtime.Scheme) error {
	fmt.Println("func myTest() was called")
	return nil
}
