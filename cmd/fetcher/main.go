package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/errors"
)

// fetch the emails
// read the emails and pass it to AI model to categorize the emails
// label emails in the user's inbox
// summarize the emails and send it to the user
// move junk emails to the spam folder
// clean up the inbox by moving emails to the archive folder after a certain period of time by user's preference
// prioritize emails based on the user's preference from a certain sender
// notify the user when an email is received from a certain sender
// notify the user when an email is received with a certain keyword
// understand user if they moved email to a folder and learn from it like spam to inbox or important to archive and etc.
// have a folder when they expect a reply from the email they sent and notify them if they didn't receive a reply after a certain period of time

type options struct {
	Username   string
	Password   string
	ImapServer string
	Port       string
	UseTLS     bool
	Unread     bool
	Today      bool
	Since      string
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

	return errors.NewAggregate(errs)
}

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	logger := logrus.WithField("component", "fetcher")

	o := gatherOptions()
	if err := o.validate(); err != nil {
		logger.WithError(err).Fatal("invalid options")
	}

	if err := connect(o); err != nil {
		logger.WithError(err).Fatal("failed to connect to the IMAP server")
	}
}

func connect(o options) error {
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

	// TODO: Remove limit processing to 10 emails for the demo
	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	messages := make(chan *imap.Message, 10)
	err = c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	if err != nil {
		return fmt.Errorf("unable to fetch emails: %v", err)
	}

	var errs []error
	emailCount := 0
	for msg := range messages {
		logrus.WithField("subject", msg.Envelope.Subject).Info("Email received")
		if err := processEmail(msg); err != nil {
			errs = append(errs, err)
		}
		emailCount++
		if emailCount >= 10 {
			break 
		}
	}

	return errors.NewAggregate(errs)
}

func processEmail(msg *imap.Message) error {
	// Dummy AI categorization (will be replaced with actual AI model call)
	category := categorizeEmail(msg.Envelope.Subject)
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

// dummy categorize email function
func categorizeEmail(subject string) string {
	if strings.Contains(strings.ToLower(subject), "offer") {
		return "junk"
	}
	return "general"
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
