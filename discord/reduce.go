package discord

import (
	"github.com/rxdn/gdl/objects/channel/message"
)

func Reduce(msg message.Message) Message {
	attachments := make([]Attachment, len(msg.Attachments))

	for i, attachment := range msg.Attachments {
		attachments[i] = Attachment{
			Filename: attachment.Filename,
			Url:      attachment.Url,
		}
	}

	return Message{
		Author: User{
			Id:       msg.Author.Id,
			Username: msg.Author.Username,
			Avatar:   msg.Author.Avatar.String(),
		},
		Content:     msg.Content,
		Timestamp:   msg.Timestamp,
		Attachments: attachments,
	}
}

func ReduceMessages(messages []message.Message) []Message {
	reduced := make([]Message, len(messages))

	for i, message := range messages {
		reduced[i] = Reduce(message)
	}

	return reduced
}
