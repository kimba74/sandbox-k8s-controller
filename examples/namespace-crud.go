package examples

import (
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sTyped "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NSClient is a structure representing an example client.
type NSClient struct {
	defaultNS string
	nsClient  k8sTyped.NamespaceInterface
}

// NewNSClient creates a new Namespace Client
func NewNSClient(defaultNS string, clientSet *kubernetes.Clientset) *NSClient {
	client := NSClient{
		defaultNS: defaultNS,
		nsClient:  clientSet.CoreV1().Namespaces(),
	}
	return &client
}

// ListAllNamespaces will retrieve all namespaces from the cluster and list them
func (nsc *NSClient) ListAllNamespaces() {
	fmt.Println("+---- Listing all Namespace resources ---------------------------")
	result, err := nsc.nsClient.List(metaV1.ListOptions{})
	handleError(err, "Error getting Namespace resource list")
	for idx, ns := range result.Items {
		fmt.Printf("| %2d. %s [%s]\n", idx+1, ns.GetName(), ns.GetUID())
	}
	fmt.Println("|")
}

// CreateNamespace creates a new Namespace resource for the provided name
func (nsc *NSClient) CreateNamespace(name string) {
	fmt.Println("+---- Create a Namespace resource -------------------------------")
	ns := &coreV1.Namespace{
		ObjectMeta: metaV1.ObjectMeta{
			Name: name,
		},
	}
	result, err := nsc.nsClient.Create(ns)
	handleError(err, "Error creating Namespace resource")
	outputNamespace(result, fmt.Sprintf("Namespace '%s' created", name))
}

// ReadNamespace reads a Namespace resource with the provided name
func (nsc *NSClient) ReadNamespace(name string) {
	fmt.Println("+---- Read a Namespace resource ---------------------------------")
	result, err := nsc.nsClient.Get(name, metaV1.GetOptions{})
	handleError(err, "Error reading Namespace resource")
	outputNamespace(result, fmt.Sprintf("Namespace '%s' read", name))
}

// UpdateNamespace updates the Namespace resource for the provided name
func (nsc *NSClient) UpdateNamespace(name string) {
	fmt.Println("+---- Update a Namespace resource -------------------------------")
	// Load the Namespace resource to update
	namespace, err := nsc.nsClient.Get(name, metaV1.GetOptions{})
	if err != nil {
		handleError(err, "Error reading Namespace resource")
		return
	}
	// Prepare a Label and an Annotation for update
	newLabels := map[string]string{"my-update": "success"}
	newAnnots := map[string]string{"update": "It worked"}
	namespace.SetLabels(newLabels)
	namespace.SetAnnotations(newAnnots)
	// Update Namespace resource
	result, err := nsc.nsClient.Update(namespace)
	handleError(err, "Error updating Namespace resource")
	outputNamespace(result, fmt.Sprintf("Namespace '%s' updated", name))
}

// DeleteNamespace deletes the Namespace resource for the provided name
func (nsc *NSClient) DeleteNamespace(name string) {
	fmt.Println("+---- Delete a Namespace resource -------------------------------")
	deletePolicy := metaV1.DeletePropagationForeground
	err := nsc.nsClient.Delete(name, &metaV1.DeleteOptions{PropagationPolicy: &deletePolicy})
	handleError(err, "Error deleting Namespace resource")
	outputNamespace(nil, fmt.Sprintf("Namespace '%s' deleted", name))
}

// RunCRUDExampe executes the full CRUD example
func (nsc *NSClient) RunCRUDExampe() {
	nsc.ListAllNamespaces()
	nsc.CreateNamespace(nsc.defaultNS)
	nsc.ReadNamespace(nsc.defaultNS)
	nsc.UpdateNamespace(nsc.defaultNS)
	nsc.DeleteNamespace(nsc.defaultNS)
	fmt.Println("+----------------------------------------------------------------")
}

func outputNamespace(namespace *coreV1.Namespace, message string) {
	fmt.Printf("| %s\n", message)
	if namespace != nil {
		fmt.Printf("|    Resource ID       : %s\n", namespace.GetUID())
		fmt.Printf("|    Resource Version  : %s\n", namespace.GetResourceVersion())
		fmt.Printf("|    Creation Timestamp: %v\n", namespace.GetCreationTimestamp())
		fmt.Printf("|    Annotations\n")
		for key, val := range namespace.GetAnnotations() {
			fmt.Printf("|      * %s = %s \n", key, val)
		}
		fmt.Printf("|    Labels\n")
		for key, val := range namespace.GetLabels() {
			fmt.Printf("|      * %s = %s \n", key, val)
		}
	}
	fmt.Printf("|\n")
}

func handleError(err error, message string) {
	if err != nil {
		fmt.Printf("| !!! %s: %s\n", message, err)
	}
}
