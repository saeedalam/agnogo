package agnogo

// HumanApproval represents a pending tool call that needs human approval.
type HumanApproval struct {
	ToolName  string            `json:"tool_name"`
	Arguments map[string]string `json:"arguments"`
	Reason    string            `json:"reason"`   // why approval is needed
	SessionID string            `json:"session_id"`
}

// Response fields for human-in-the-loop:
//   NeedsApproval: true when a tool requires human approval
//   Approval: the pending approval request (tool + args + reason)
//
// Flow:
//   1. Agent calls a tool with RequireApproval=true
//   2. Run returns Response{NeedsApproval: true, Approval: {...}}
//   3. Your app shows the approval to a human
//   4. Human approves/rejects
//   5. Call agent.Resume(ctx, session, approved) to continue
