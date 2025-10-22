package utils

// ShouldNotifySlack determines if a severity level should trigger Slack notification
func ShouldNotifySlack(severity string) bool {
	return severity == "P0" || severity == "P1" || severity == "P2" || severity == "P3"
}

// ShouldNotifySlackForResolution determines if resolution should be sent to Slack
func ShouldNotifySlackForResolution(severity string) bool {
	return severity == "P0" || severity == "P1" || severity == "P2"
}

// GetChannelForSeverity returns the appropriate Slack channel based on alert severity
func GetChannelForSeverity(severity string, channels map[string]string) string {
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
	return channels["default"]
}
