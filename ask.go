package agnogo

import "context"

// Ask sends a one-shot message using an ephemeral session and returns the
// response text. This is the simplest way to get a single answer from an agent.
//
//	answer, err := agent.Ask(ctx, "What is the capital of France?")
func (a *Core) Ask(ctx context.Context, msg string) (string, error) {
	session := NewSession(generateRunID())
	resp, err := a.Run(ctx, session, msg)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// AskStream sends a message using an ephemeral session and returns a streaming
// channel of response chunks. The channel closes when the response is complete.
//
//	for chunk := range agent.AskStream(ctx, "Tell me a story") {
//	    fmt.Print(chunk.Text)
//	}
func (a *Core) AskStream(ctx context.Context, msg string) <-chan StreamChunk {
	session := NewSession(generateRunID())
	return a.RunStream(ctx, session, msg)
}

// AskStructured sends a one-shot message and parses the response into the
// provided struct. Uses an ephemeral session.
//
//	var result MyStruct
//	err := agnogo.AskStructured(ctx, agent, "Extract info from: ...", &result)
func AskStructured[T any](ctx context.Context, agent *Core, msg string, out *T) error {
	session := NewSession(generateRunID())
	return RunStructured(ctx, agent, session, msg, out)
}
