package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
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
		go watchNamespace(clientSet, ns, alertRepo, slackClient)
	}
}

func watchNamespace(clientSet *kubernetes.Clientset, namespace string, alertRepo *repository.PostgresAlertRepository, slackCient *slack.Client) {
	log.Printf("Starting watcher for namespace: %s", namespace)

	for {
		if err := watchLoop(clientSet, namespace, alertRepo, slackCient); err != nil {
			log.Printf("Watch error in %s: %v. Reconnecting in 10s...", namespace, err)
			time.Sleep(10 * time.Second)
		}
	}

}

func watchLoop(clientSet *kubernetes.Clientset, namespace string, alertRepo *repository.PostgresAlertRepository, slackClient *slack.Client) error {

	watcher, err := clientSet.CoreV1().Pods(namespace).Watch(
		context.Background(),
		metav1.ListOptions{},
	)

	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		handleEvent(event, namespace, alertRepo, slackClient)
	}

	return nil
}

func handleEvent(event watch.Event, namespace string, alertRepo *repository.PostgresAlertRepository, slackClient *slack.Client) {
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
			handleExistingAlert(pod, existingAlert, alertRepo, slackClient)
		} else {
			// No alert - check if should create
			if shouldAlert(pod, namespace) {
				createAlert(pod, alertRepo, slackClient)
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

func createAlert(pod *v1.Pod, alertRepo *repository.PostgresAlertRepository, slackClient *slack.Client) {
	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}
	labels["pod_name"] = pod.Name
	labels["node_name"] = pod.Spec.NodeName

	severity := getSeverity(pod)

	request := models.CreateAlertRequest{
		Source:      "kubernetes",
		Severity:    severity,
		Title:       buildTitle(pod),
		Description: buildDescription(pod),
		Namespace:   pod.Namespace,
		Labels:      labels,
		TraceID:     generateTraceID(),
	}

	// Save to database
	response, err := alertRepo.Create(request)
	if err != nil {
		// Check if it's a duplicate error
		if strings.Contains(err.Error(), "duplicate") {
			log.Printf("⚠️ Alert already exists for pod %s", pod.Name)
			return
		}
		log.Printf("❌ Failed to create alert for pod %s: %v", pod.Name, err)
		return
	}

	log.Printf("🚨 ALERT CREATED: %s (ID: %s)", pod.Name, response.ID)

	channels := slackClient.GetChannels()

	if shouldNotifySlack(severity) && slackClient != nil {
		channel := getChannelForSeverity(severity, channels)
		log.Printf("Send alert into %s channel, severity: %s", channel, severity)
		err := notifySlack(slackClient, request, response, severity, channel)
		if err != nil {
			log.Printf("⚠️ Slack notification failed for alert %s: %v", response.ID, err)
		}
	} else {
		log.Printf("Nothing to send into slack [shouldNotifySlack: %w]", shouldNotifySlack(severity))
	}

}

func shouldNotifySlack(severity string) bool {
	// Configure which severities trigger Slack
	return severity == "P0" || severity == "P1" || severity == "P2"
}

func notifySlack(client *slack.Client, request models.CreateAlertRequest, response models.CreateAlertResponse, severity string, channelName string) error {
	// Build message with full details
	message := fmt.Sprintf(
		"🚨 *%s Alert Created*\n"+
			"*ID:* %s\n"+
			"*Title:* %s\n"+
			"*Namespace:* %s\n"+
			"*Description:* %s\n"+
			"*Status:* %s\n"+
			"*Created:* %s",
		severity,
		response.ID,
		request.Title,       // From request
		request.Namespace,   // From request
		request.Description, // From request
		response.Status,     // From response
		response.CreatedAt,  // From response
	)

	return client.SendMessageToChannel(message, channelName)
}

func notifySlackWithResolving(client *slack.Client, alert *models.Alert, severity string, channelName string) error {
	// Build message with full details
	message := fmt.Sprintf(
		"🚨 *%s Alert Resolved*\n"+
			"*ID:* %s\n"+
			"*Title:* %s\n"+
			"*Namespace:* %s\n"+
			"*Description:* %s\n"+
			"*Status:* %s\n"+
			"*Resolved at:* %s",
		severity,
		alert.ID,
		alert.Title,
		alert.Namespace,
		alert.Description,
		alert.Status,
		alert.ResolvedAt,
	)

	return client.SendMessageToChannel(message, channelName)
}

func generateTraceID() string {
	// Generate unique trace ID for correlation
	return uuid.New().String()
}

func getSeverity(pod *v1.Pod) string {
	name := pod.Name
	if strings.Contains(name, "kafka") || strings.Contains(name, "postgres") || strings.Contains(name, "agent") {
		return "P1"
	} else if strings.Contains(name, "aggregator") {
		return "P2"
	} else {
		return "P3"
	}
}

func getChannelForSeverity(severity string, channels map[string]string) string {
	switch severity {
	case "P0", "P1":
		if ch, ok := channels["critical"]; ok {
			return ch
		}
	case "P2", "P3":
		if ch, ok := channels["default"]; ok {
			return ch
		}
	}
	return channels["default"] // Fallback
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

func handleExistingAlert(pod *v1.Pod, alert *models.Alert, repo *repository.PostgresAlertRepository, slackClient *slack.Client) {
	// Check if pod is healthy now
	if isHealthy(pod) {
		// Resolve the alert
		resolvedAlert, err := repo.Resolve(alert.ID)
		if err != nil {
			log.Printf("❌ Failed to resolve alert: %v", err)
			return
		}
		log.Printf("✅ Alert RESOLVED: %s (ID: %s)", pod.Name, resolvedAlert.ID)

		// Only notify Slack for important severities
		severity := alert.Severity
		if shouldNotifySlackForResolution(severity) && slackClient != nil {
			channel := getChannelForSeverity(severity, slackClient.GetChannels())
			err := notifySlackWithResolving(slackClient, resolvedAlert, severity, channel)
			if err != nil {
				log.Printf("⚠️ Slack notification failed for resolved alert %s: %v", alert.ID, err)
			}
		}
	} else {
		// Still unhealthy, do nothing
		log.Printf("⏳ Pod %s still unhealthy, alert remains open", pod.Name)
	}
}

func shouldNotifySlackForResolution(severity string) bool {
	return severity == "P0" || severity == "P1" || severity == "P2"
}

func isHealthy(pod *v1.Pod) bool {
	// More comprehensive health check
	if pod.Status.Phase != v1.PodRunning {
		return false
	}

	// Check if ALL containers are ready
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status != v1.ConditionTrue {
			return false
		}
	}

	return true
}
