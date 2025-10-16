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
	client  *slack.Client
	channel string
}

func NewSlackClient(cfg config.SlackConfig) (*Client, error) {
	token := config.GetSlackToken()

	if !strings.HasPrefix(token, "xoxb-") {
		log.Printf("SLACK_BOT_TOKEN must be a bot token (xoxb-)!!!")
		os.Exit(1)
	}

	api := slack.New(token)
	_, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Slack: %w", err)
	}

	log.Printf("✅ Successfully connected to Slack, channel: %s", cfg.ChannelName)

	return &Client{
		client:  api,
		channel: cfg.ChannelName,
	}, nil

}

func (c *Client) SendMessage(text string) error {
	_, _, err := c.client.PostMessage(c.channel,
		slack.MsgOptionText(text, false),
		slack.MsgOptionUsername("K8s Bot"),
		slack.MsgOptionIconEmoji(":alert:"),
	)
	if err != nil {
		log.Printf("ERROR: Could not send message to %s: %w", c.channel, err)
	}

	log.Printf("📤 Sent message to Slack channel %s", c.channel)
	return nil
}
