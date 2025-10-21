package slack

import (
	"fmt"
	"gomon/alerting/internal/config"
	"log"
	"os"

	"strings"

	"github.com/slack-go/slack"
)

type Client struct {
	client         *slack.Client
	channels       map[string]string
	circuitBreaker *CircuitBreaker
}

func NewSlackClient(cfg config.SlackConfig) (*Client, error) {
	token := config.GetSlackToken()

	circuitBreaker := NewCircuitBreaker(cfg.CircuitBreaker)

	if !strings.HasPrefix(token, "xoxb-") {
		log.Printf("SLACK_BOT_TOKEN must be a bot token (xoxb-)!!!")
		os.Exit(1)
	}

	api := slack.New(token)
	_, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Slack: %w", err)
	}

	log.Printf("✅ Successfully connected to Slack")

	return &Client{
		client:         api,
		channels:       cfg.Channels,
		circuitBreaker: circuitBreaker,
	}, nil

}

func (c *Client) SendMessage(text string) error {
	if !c.circuitBreaker.canExecute() {
		log.Printf("⚠️ Circuit breaker is OPEN, skipping Slack notification")
		return fmt.Errorf("circuit breaker open: Slack temporarily unavailable")
	}

	_, _, err := c.client.PostMessage(c.channels["default"],
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		c.circuitBreaker.recordFailure()
		log.Printf("❌ Slack API failed (failures: %d)", c.circuitBreaker.GetFailureCount())
		return fmt.Errorf("failed to send message to %s: %w", c.channels["default"], err)
	}

	c.circuitBreaker.recordSuccess()

	log.Printf("📤 Sent message to Slack channel %s", c.channels["default"])
	return nil
}

func (c *Client) SendMessageToChannel(text string, channelName string) error {
	if !c.circuitBreaker.canExecute() {
		log.Printf("⚠️ Circuit breaker is OPEN, skipping Slack notification")
		return fmt.Errorf("circuit breaker open: Slack temporarily unavailable")
	}

	_, _, err := c.client.PostMessage(channelName,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		c.circuitBreaker.recordFailure()
		log.Printf("❌ Slack API failed (failures: %d)", c.circuitBreaker.GetFailureCount())
		log.Printf("ERROR: Could not send message to %s: %v", channelName, err)
	}

	c.circuitBreaker.recordSuccess()

	log.Printf("📤 Sent message to Slack channel %s", channelName)
	return nil
}

func (c *Client) GetChannels() map[string]string {
	return c.channels
}
