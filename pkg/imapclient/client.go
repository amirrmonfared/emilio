package imapclient

import (
	"context"
	"fmt"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"

	"github.com/sirupsen/logrus"

	"github.com/amirrmonfared/emilio/pkg/config"
	"github.com/amirrmonfared/emilio/pkg/processor"
)

func ConnectAndProcessEmails(ctx context.Context, o config.Options, p *processor.EmailProcessor) error {
	c, err := client.DialTLS(fmt.Sprintf("%s:%s", o.ImapServer, o.Port), nil)
	if err != nil {
		return fmt.Errorf("unable to connect to the IMAP server: %v", err)
	}
	defer c.Logout()

	if err := c.Login(o.Username, o.Password); err != nil {
		return fmt.Errorf("unable to login: %v", err)
	}
	logrus.Info("Logged in to the IMAP server")

	return fetchAndProcessEmails(ctx, c, o, p)
}

func fetchAndProcessEmails(ctx context.Context, c *client.Client, o config.Options, p *processor.EmailProcessor) error {
	_, err := c.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("unable to select INBOX: %v", err)
	}

	criteria := buildSearchCriteria(o)

	ids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("unable to search emails: %v", err)
	}
	if len(ids) == 0 {
		logrus.Debug("No emails found with the specified criteria")
		return nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	messages := make(chan *imap.Message, 10)
	err = c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchBody}, messages)
	if err != nil {
		return fmt.Errorf("unable to fetch emails: %v", err)
	}

	return p.ProcessMessageBatch(ctx, messages)
}

func buildSearchCriteria(o config.Options) *imap.SearchCriteria {
	criteria := imap.NewSearchCriteria()
	if o.Unread {
		criteria.WithoutFlags = []string{"\\Seen"}
	}
	if o.Today {
		today := time.Now().Truncate(24 * time.Hour)
		criteria.Since = today
	}
	if o.Since != "" {
		if parsedDate, err := time.Parse("2006-01-02", o.Since); err == nil {
			criteria.Since = parsedDate
		}
	}
	return criteria
}
