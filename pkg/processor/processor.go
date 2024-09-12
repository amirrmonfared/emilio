package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/sirupsen/logrus"

	"github.com/amirrmonfared/emilio/pkg/openai"
)

type EmailProcessor struct {
	llm    openai.OpenAI
	logger *logrus.Entry
}

// Create a new EmailProcessor
func New(logger *logrus.Entry, llm openai.OpenAI) *EmailProcessor {
	return &EmailProcessor{
		llm:    llm,
		logger: logger,
	}
}

// Process a batch of messages
func (ep *EmailProcessor) ProcessMessageBatch(ctx context.Context, messages chan *imap.Message) error {
	var errs []error
	emailCount := 0
	for msg := range messages {
		if err := ep.processSingleEmail(ctx, msg); err != nil {
			errs = append(errs, err)
		}
		emailCount++
		if emailCount >= 10 {
			break
		}
	}
	return nil
}

// Process a single email
func (ep *EmailProcessor) processSingleEmail(ctx context.Context, msg *imap.Message) error {
	emailContent := ep.buildEmailContent(msg)
	category := ep.categorizeEmail(ctx, emailContent)
	ep.logger.WithField("subject", msg.Envelope.Subject).WithField("category", category).Info("Categorized email")

	if ep.isPriorityEmail(msg) {
		ep.notifyUser(fmt.Sprintf("Priority email from: %v", msg.Envelope.From))
	}

	if category == "junk" {
		ep.moveToFolder(msg, "Spam")
	} else {
		ep.moveToFolder(msg, "Archive")
	}
	return nil
}

// Build email content
func (ep *EmailProcessor) buildEmailContent(msg *imap.Message) string {
	return fmt.Sprintf("Subject: %s\nFrom: %v\nDate: %v\n", msg.Envelope.Subject, msg.Envelope.From, msg.Envelope.Date)
}

// Categorize email using OpenAI (continued)
func (ep *EmailProcessor) categorizeEmail(ctx context.Context, emailContent string) string {
	ep.logger.Debug("Calling OpenAI API for email categorization")
	completion, err := ep.llm.Call(ctx, emailContent, openai.WithTemperature(0.8))
	if err != nil {
		ep.logger.Fatalf("Failed to categorize email: %v", err)
	}
	return strings.TrimSpace(completion)
}

// Check if the email is from a priority sender
func (ep *EmailProcessor) isPriorityEmail(msg *imap.Message) bool {
	prioritySenders := []string{"important@company.com"}
	return ep.containsSender(prioritySenders, msg.Envelope.From)
}

// Check if the sender is in the priority list
func (ep *EmailProcessor) containsSender(prioritySenders []string, from []*imap.Address) bool {
	for _, addr := range from {
		for _, sender := range prioritySenders {
			if strings.Contains(addr.Address(), sender) {
				return true
			}
		}
	}
	return false
}

// Notify the user about important emails
func (ep *EmailProcessor) notifyUser(message string) {
	ep.logger.Warn(message)
}

// Move the email to the specified folder
func (ep *EmailProcessor) moveToFolder(msg *imap.Message, folder string) {
	ep.logger.Infof("Moving email %s to folder: %s", msg.Envelope.Subject, folder)
}
