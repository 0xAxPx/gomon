package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"gomon/alerting/internal/models"
	"gomon/alerting/internal/repository"
	"gomon/alerting/internal/slack"
)

func StartWatching(clientSet *kubernetes.Clientset, alertRepo *repository.PostgresAlertRepository, slackClient *slack.Client) {
	namespaces := []string{"monitoring", "kube-system", "ingress-nginx"}

	for _, ns := range namespaces {
		// Run each watcher concurrently
		go watchNamespace(clientSet, ns, alertRepo)
	}
}

func watchNamespace(clientSet *kubernetes.Clientset, namespace string, alertRepo *repository.PostgresAlertRepository) {
	log.Printf("Starting watcher for namespace: %s", namespace)

	for {
		if err := watchLoop(clientSet, namespace, alertRepo); err != nil {
			log.Printf("Watch error in %s: %v. Reconnecting in 10s...", namespace, err)
			time.Sleep(10 * time.Second)
		}
	}

}

func watchLoop(clientSet *kubernetes.Clientset, namespace string, alertRepo *repository.PostgresAlertRepository) error {

	watcher, err := clientSet.CoreV1().Pods(namespace).Watch(
		context.Background(),
		metav1.ListOptions{},
	)

	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		handleEvent(event, namespace, alertRepo)
	}

	return nil
}

func handleEvent(event watch.Event, namespace string, alertRepo *repository.PostgresAlertRepository) {
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

		existingAlert, err := alertRepo.FindActiveAlertByPod(namespace, pod.Name)
		if err != nil {
			log.Printf("Error checking for existing alert: %v", err)
			return
		}

		if existingAlert != nil {
			// Alert exists - handle it
			handleExistingAlert(pod, existingAlert, alertRepo)
		} else {
			// No alert - check if should create
			if shouldAlert(pod, namespace) {
				createAlert(pod, alertRepo)
			}
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

func createAlert(pod *v1.Pod, alertRepo *repository.PostgresAlertRepository) {
	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}
	labels["pod_name"] = pod.Name

	request := models.CreateAlertRequest{
		Source:      "kubernetes",
		Severity:    getSeverity(pod),
		Title:       buildTitle(pod),
		Description: buildDescription(pod),
		Namespace:   pod.Namespace,
		Labels:      labels,
		TraceID:     "",
	}

	response, err := alertRepo.Create(request)
	if err != nil {
		log.Printf("❌ Failed to create alert for pod %s: %v", pod.Name, err)
		return
	}

	log.Printf("🚨 ALERT CREATED: %s (ID: %s)", pod.Name, response.ID)
}

func getSeverity(pod *v1.Pod) string {
	name := pod.Name
	if strings.Contains(name, "kafka") || strings.Contains(name, "postgres") {
		return "P1"
	} else if strings.Contains(name, "aggregator") {
		return "P2"
	} else {
		return "P3"
	}
}

func buildTitle(pod *v1.Pod) string {
	if pod.Status.Phase != v1.PodRunning {
		return fmt.Sprintf("Pod %s is %s", pod.Name, pod.Status.Phase)
	}

	restarts := getPodRestarts(pod)
	return fmt.Sprintf("Pod %s has %d restarts", pod.Name, restarts)
}

func buildDescription(pod *v1.Pod) string {
	parts := []string{
		fmt.Sprintf("Pod: %s", pod.Name),
		fmt.Sprintf("Namespace: %s", pod.Namespace),
		fmt.Sprintf("Phase: %s", pod.Status.Phase),
		fmt.Sprintf("Restarts: %d", getPodRestarts(pod)),
	}

	if pod.Status.Reason != "" {
		parts = append(parts, fmt.Sprintf("Reason: %s", pod.Status.Reason))
	}

	if pod.Status.Message != "" {
		parts = append(parts, fmt.Sprintf("Message: %s", pod.Status.Message))
	}

	return strings.Join(parts, "\n")
}

func handleExistingAlert(pod *v1.Pod, alert *models.Alert, repo *repository.PostgresAlertRepository) {
	// Check if pod is healthy now
	if isHealthy(pod) {
		// Resolve the alert
		resolvedAlert, err := repo.Resolve(alert.ID)
		if err != nil {
			log.Printf("❌ Failed to resolve alert: %v", err)
			return
		}
		log.Printf("✅ Alert RESOLVED: %s (ID: %s)", pod.Name, resolvedAlert.ID)
	} else {
		// Still unhealthy, do nothing
		log.Printf("⏳ Pod %s still unhealthy, alert remains open", pod.Name)
	}
}

func isHealthy(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning
}
