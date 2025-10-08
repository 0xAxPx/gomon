package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ListPods(clientSet *kubernetes.Clientset, namespace string) {

	pods, errors := clientSet.CoreV1().Pods(namespace).List(
		context.TODO(), metav1.ListOptions{},
	)

	if errors != nil {
		fmt.Printf("Error listing pods: %v\n", errors)
		return
	}

	fmt.Printf("\nFound %d pods in namespace '%s':\n", len(pods.Items), namespace)

	for _, pod := range pods.Items {
		fmt.Printf("- %s: Phase=%s, Restarts=%d\n",
			pod.Name,
			pod.Status.Phase,
			getPodRestartCount(pod),
		)
	}
}

func getPodRestartCount(pod v1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}

	return restarts
}
