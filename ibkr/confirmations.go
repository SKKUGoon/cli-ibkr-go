package ibkr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Answers map[string]bool

func DefaultAnswers() Answers {
	return Answers{
		"o163": true, "price exceeds the Percentage constraint": true,
		"o451": true, "exceeds the Total Value Limit": true,
		"o354": true, "You are submitting an order without market data": true,
		"o10331": true, "You are about to submit a stop order": true,
	}
}
func (answers Answers) Find(message, id string) (bool, bool) {
	if answer, ok := answers[id]; ok {
		return answer, true
	}
	for _, key := range sortedKeys(answers) {
		if strings.Contains(message, key) {
			return answers[key], true
		}
	}
	return false, false
}

type OrderPrompt struct{ ReplyID, Message, MessageID string }

func scalarString(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}
func ParseOrderPrompt(response json.RawMessage) (*OrderPrompt, error) {
	var items []map[string]json.RawMessage
	if len(response) == 0 || strings.TrimSpace(string(response))[0] != '[' || json.Unmarshal(response, &items) != nil {
		return nil, fmt.Errorf("unexpected order response: %s", response)
	}
	if len(items) == 0 {
		return nil, nil
	}
	first := items[0]
	messages, exists := first["message"]
	if !exists {
		return nil, nil
	}
	var message string
	if json.Unmarshal(messages, &message) != nil {
		var list []string
		if json.Unmarshal(messages, &list) != nil || len(list) == 0 {
			return nil, fmt.Errorf("unexpected order message: %s", response)
		}
		message = list[0]
	}
	replyID := scalarString(first["id"])
	if replyID == "" {
		return nil, fmt.Errorf("missing order reply id: %s", response)
	}
	id := scalarString(first["messageId"])
	if id == "" {
		var ids []json.RawMessage
		if json.Unmarshal(first["messageIds"], &ids) == nil && len(ids) > 0 {
			id = scalarString(ids[0])
		}
	}
	return &OrderPrompt{replyID, strings.ReplaceAll(strings.TrimSpace(message), "\n", ""), id}, nil
}
func FinalOrderResponse(response json.RawMessage) json.RawMessage {
	var items []json.RawMessage
	if json.Unmarshal(response, &items) == nil && len(items) == 1 {
		return items[0]
	}
	return response
}
func (client *Client) Reply(ctx context.Context, id string, confirmed bool) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", ReplyPath(id), nil, map[string]bool{"confirmed": confirmed})
}
func (client *Client) HandleConfirmations(ctx context.Context, response json.RawMessage, answers Answers, maxReplies uint32) (json.RawMessage, error) {
	for count := uint32(0); count < maxReplies; count++ {
		prompt, err := ParseOrderPrompt(response)
		if err != nil {
			return nil, err
		}
		if prompt == nil {
			return FinalOrderResponse(response), nil
		}
		answer, known := answers.Find(prompt.Message, prompt.MessageID)
		if !known {
			return nil, fmt.Errorf("missing order answer: %s (messageId: %s)", prompt.Message, prompt.MessageID)
		}
		if !answer {
			return nil, fmt.Errorf("rejected order answer: %s (messageId: %s)", prompt.Message, prompt.MessageID)
		}
		response, err = client.Reply(ctx, prompt.ReplyID, true)
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("too many order replies (%d): %s", maxReplies, response)
}
