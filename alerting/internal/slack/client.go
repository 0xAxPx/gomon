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
	client   *slack.Client
	channels map[string]string
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

	log.Printf("✅ Successfully connected to Slack")

	return &Client{
		client:   api,
		channels: cfg.Channels,
	}, nil

}

func (c *Client) SendMessage(text string) error {
	_, _, err := c.client.PostMessage(c.channels["default"],
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		log.Printf("ERROR: Could not send message to %s: %w", c.channels["defaul"], err)
	}

	log.Printf("📤 Sent message to Slack channel %s", c.channels["default"])
	return nil
}

func (c *Client) SendMessageToChannel(text string, channelName string) error {
	_, _, err := c.client.PostMessage(channelName,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		log.Printf("ERROR: Could not send message to %s: %w", channelName, err)
	}

	log.Printf("📤 Sent message to Slack channel %s", channelName)
	return nil
}

func (c *Client) GetChannels() map[string]string {
	return c.channels
}
