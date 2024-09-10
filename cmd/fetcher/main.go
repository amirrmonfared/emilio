package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/errors"
)

type options struct {
	Username   string
	Password   string
	ImapServer string
	Port       string
	UseTLS     bool
	Unread     bool
	Today      bool
	Since      string
	APIKey     string
	Model      string
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.Username, "username", "", "IMAP username")
	fs.StringVar(&o.Password, "password", "", "IMAP password")
	fs.StringVar(&o.ImapServer, "imap-server", "", "IMAP server")
	fs.StringVar(&o.Port, "port", "993", "IMAP port")
	fs.BoolVar(&o.UseTLS, "use-tls", true, "Use TLS")
	fs.BoolVar(&o.Unread, "unread", false, "Fetch only unread emails")
	fs.BoolVar(&o.Today, "today", false, "Fetch only emails received today")
	fs.StringVar(&o.Since, "since", "", "Fetch emails since a specific date (YYYY-MM-DD)")
	fs.StringVar(&o.APIKey, "api-key", "", "OpenAI API Key")
	fs.StringVar(&o.Model, "model", "gpt-4o-mini", "OpenAI model to use")

	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("failed to parse the arguments")
	}

	return o
}

func (o options) validate() error {
	var errs []error

	if o.Username == "" {
		errs = append(errs, fmt.Errorf("username is required"))
	}
	if o.Password == "" {
		errs = append(errs, fmt.Errorf("password is required"))
	}
	if o.ImapServer == "" {
		errs = append(errs, fmt.Errorf("IMAP server is required"))
	}
	if o.APIKey == "" {
		errs = append(errs, fmt.Errorf("OpenAI API Key is required"))
	}

	return errors.NewAggregate(errs)
}

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	logger := logrus.WithField("component", "fetcher")

	o := gatherOptions()
	if err := o.validate(); err != nil {
		logger.WithError(err).Fatal("invalid options")
	}

	ctx := context.Background()

	llm, err := openai.New(openai.WithToken(o.APIKey))
	if err != nil {
		log.Fatalf("Failed to initialize LLM: %v", err)
	}

	if err := connectAndProcessEmails(ctx, o, llm); err != nil {
		logger.WithError(err).Fatal("failed to process emails")
	}
}

func connectAndProcessEmails(ctx context.Context, o options, llm llms.Model) error {
	c, err := client.DialTLS(fmt.Sprintf("%s:%s", o.ImapServer, o.Port), nil)
	if err != nil {
		return fmt.Errorf("unable to connect to the IMAP server: %v", err)
	}
	defer c.Logout()

	if err := c.Login(o.Username, o.Password); err != nil {
		return fmt.Errorf("unable to login: %v", err)
	}
	logrus.Info("Logged in to the IMAP server")

	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("unable to select INBOX: %v", err)
	}
	logrus.WithField("mailbox", mbox).Info("Mailbox status")

	criteria := imap.NewSearchCriteria()

	if o.Unread {
		criteria.WithoutFlags = []string{"\\Seen"}
	}
	if o.Today {
		today := time.Now().Truncate(24 * time.Hour)
		criteria.Since = today
	}
	if o.Since != "" {
		parsedDate, err := time.Parse("2006-01-02", o.Since)
		if err != nil {
			return fmt.Errorf("invalid date format for 'since' argument: %v", err)
		}
		criteria.Since = parsedDate
	}

	ids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("unable to search emails: %v", err)
	}
	if len(ids) == 0 {
		logrus.Info("No emails found with the specified criteria")
		return nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	messages := make(chan *imap.Message, 10)
	err = c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchBody}, messages)
	if err != nil {
		return fmt.Errorf("unable to fetch emails: %v", err)
	}

	var errs []error
	emailCount := 0
	for msg := range messages {
		logrus.WithField("subject", msg.Envelope.Subject).Info("Email received")
		if err := processEmail(ctx, msg, llm); err != nil {
			errs = append(errs, err)
		}
		emailCount++
		if emailCount >= 10 {
			break
		}
	}

	return errors.NewAggregate(errs)
}

func processEmail(ctx context.Context, msg *imap.Message, llm llms.Model) error {
	emailContent := fmt.Sprintf("Subject: %s\nFrom: %v\nDate: %v\n", msg.Envelope.Subject, msg.Envelope.From, msg.Envelope.Date)

	category := categorizeEmail(ctx, emailContent, llm)
	logrus.WithField("subject", msg.Envelope.Subject).WithField("category", category).Info("Categorized email")

	prioritySenders := []string{"important@company.com"}
	importantKeywords := []string{"urgent", "immediate action"}

	if containsSender(prioritySenders, msg.Envelope.From) {
		notifyUser(fmt.Sprintf("Priority email from: %v", msg.Envelope.From))
	} else if containsKeyword(importantKeywords, msg.Envelope.Subject) {
		notifyUser(fmt.Sprintf("Email with important keyword received: %s", msg.Envelope.Subject))
	}

	if category == "junk" {
		moveToFolder(msg, "Spam")
	} else {
		moveToFolder(msg, "Archive")
	}
	return nil
}

func categorizeEmail(ctx context.Context, emailContent string, llm llms.Model) string {
	completion, err := llm.Call(ctx, emailContent, llms.WithTemperature(0.8))
	if err != nil {
		log.Fatalf("Failed to categorize email: %v", err)
	}

	category := strings.TrimSpace(completion)
	log.Printf("Categorized email as: %s", category)
	return category
}

func containsSender(prioritySenders []string, from []*imap.Address) bool {
	for _, addr := range from {
		for _, sender := range prioritySenders {
			if strings.Contains(addr.Address(), sender) {
				return true
			}
		}
	}
	return false
}

func containsKeyword(keywords []string, subject string) bool {
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(subject), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func notifyUser(message string) {
	logrus.Warn(message)
}

func moveToFolder(msg *imap.Message, folder string) {
	fmt.Printf("Moving email %s to folder: %s\n", msg.Envelope.Subject, folder)
}
