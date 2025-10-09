package k8s

import (
	"context"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func StartWatching(clientSet *kubernetes.Clientset) {
	namespaces := []string{"monitoring", "kube-system", "ingress-nginx"}

	for _, ns := range namespaces {
		// Run each watcher concurrently
		go watchNamespace(clientSet, ns)
	}
}

func watchNamespace(clientSet *kubernetes.Clientset, namespace string) {
	log.Printf("Starting watcher for namespace: %s", namespace)

	for {
		if err := watchLoop(clientSet, namespace); err != nil {
			log.Printf("Watch error in %s: %v. Reconnecting in 10s...", namespace, err)
			time.Sleep(10 * time.Second)
		}
	}

}

func watchLoop(clientSet *kubernetes.Clientset, namespace string) error {

	watcher, err := clientSet.CoreV1().Pods(namespace).Watch(
		context.Background(),
		metav1.ListOptions{},
	)

	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		handleEvent(event, namespace)
	}

	return err

}

func handleEvent(event watch.Event, namespace string) {
	pod, ok := event.Object.(*v1.Pod)
	if !ok {
		return
	}

	switch event.Type {

	case watch.Added:
		log.Printf("[%s] Pod ADDED: %s", namespace, pod.Name)

	case watch.Modified:
		log.Printf("[%s] Pod MODIFIED: %s (Phase=%s, Restarts=%d)",
			namespace, pod.Name, pod.Status.Phase, getPodRestarts(pod))

		// Check if we should create alert
		if shouldAlert(pod, namespace) {
			createAlert(pod)
		}
	case watch.Deleted:
		log.Printf("[%s] Pod DELETED: %s", namespace, pod.Name)
	}

}

func shouldAlert(pod *v1.Pod, namespace string) bool {
	if pod.Status.Phase != v1.PodRunning {
		return true
	}

	restarts := getPodRestarts(pod)

	// Different thresholds per namespace
	switch namespace {
	case "kube-system":
		return restarts > 10
	case "monitoring", "ingress-nginx":
		return restarts > 3
	default:
		return restarts > 5
	}
}

func getPodRestarts(pod *v1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}

	return restarts
}

func createAlert(pod *v1.Pod) {
	log.Printf("🚨 ALERT: Pod %s needs attention!", pod.Name)
}
