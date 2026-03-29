package tools

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/saeedalam/agnogo"
)

// Email returns a tool for sending emails via SMTP.
func Email(smtpHost string, smtpPort int, username, password, fromAddr string) []agnogo.ToolDef {
	return []agnogo.ToolDef{{
		Name: "send_email", Desc: "Send an email",
		Params: agnogo.Params{
			"to":      {Type: "string", Desc: "Recipient email", Required: true},
			"subject": {Type: "string", Desc: "Email subject", Required: true},
			"body":    {Type: "string", Desc: "Email body", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			to := args["to"]
			subject := args["subject"]
			body := args["body"]

			msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
				fromAddr, to, subject, body)

			addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)
			auth := smtp.PlainAuth("", username, password, smtpHost)

			if err := smtp.SendMail(addr, auth, fromAddr, []string{to}, []byte(msg)); err != nil {
				return fmt.Sprintf("Failed to send email: %s", err), nil
			}
			return fmt.Sprintf("Email sent to %s: %s", to, subject), nil
		},
	}}
}
