package main

import (
	"fmt"

	"github.com/codewandler/agentapis/api/openrouter"
	"github.com/codewandler/agentapis/conversation"
)

func main() {
	sess := conversation.New(
		nil,
		conversation.WithModel("openai/gpt-4o-mini"),
		conversation.WithMessageProjector(openrouter.ConversationProjector{}),
	)

	req := conversation.NewRequest().User("hello").Build()
	msgs, err := sess.ProjectMessages(req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("projected messages: %d\n", len(msgs))

	built, err := sess.BuildRequest(req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("built request model: %s messages: %d\n", built.Model, len(built.Messages))
}
