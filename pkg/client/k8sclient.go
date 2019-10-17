package client

/*import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/client-go/rest"

	//"k8s.io/client-go/kubernetes"
	//"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sClient struct {
	api corev1.CoreV1Interface
}

func NewK8sClient(kubeconfigFile string) (c *K8sClient, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return
	}
	client, err := rest.RESTClientFor(config)
	if err != nil {
		return
	}
	//clientset, err := kubernetes.NewForConfig(config)
	//if err != nil {
	//	return
	//}
	clientset := corev1.New(client)
	return &K8sClient{clientset}, nil
}

func (c *K8sClient) getObjects(ctx context.Context, ns string, args []string) (ch chan interface{}, err error) {
	listOptions := metav1.ListOptions{LabelSelector: ""}
	_, err = c.api.PersistentVolumeClaims(ns).List(listOptions)
	err = errors.Wrap(err, "list")
	return
}

func (c *K8sClient) Watch(ns string) (ch <-chan watch.Event, err error) {
	listOptions := metav1.ListOptions{LabelSelector: ""}
	watcher, err := c.api.PersistentVolumeClaims(ns).Watch(listOptions)
	if err != nil {
		return
	}
	ch = watcher.ResultChan()
	for event := range ch {
		switch event.Type {
		case watch.Added:
			fmt.Printf("## Added: %#v\n", event.Object)
		case watch.Deleted:
			fmt.Printf("## Deleted: %#v\n", event.Object)
		}
	}
	return
}
*/
