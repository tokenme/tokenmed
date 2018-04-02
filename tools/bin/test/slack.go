package main

import (
	"fmt"
	"github.com/nlopes/slack"
)

const TOKEN = "xoxp-340014960567-338241194720-339563622341-94fcb61ce9353b2b0f5a86d4e99580d8"

func main() {
	client := slack.New(TOKEN)
	client.SetDebug(true)
	params := slack.PostMessageParameters{}
	attachment := slack.Attachment{
		AuthorId:   "122343",
		AuthorName: "Syd Xu",
		AuthorIcon: "https://tokenmama.io/user/avatar/+86133423423",
		Title:      "some title",
		TitleLink:  "https://tokenmama.io",
		Pretext:    "some pretext",
		Text:       "some text",
		Fields: []slack.AttachmentField{
			slack.AttachmentField{
				Title: "a",
				Value: "no",
				Short: true,
			},
			slack.AttachmentField{
				Title: "b",
				Value: "yes",
				Short: true,
			},
		},
	}
	params.Attachments = []slack.Attachment{attachment}
	channelID, timestamp, err := client.PostMessage("G9Y7METUG", "Some message ..adsfsjdfas", params)
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	fmt.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
}
