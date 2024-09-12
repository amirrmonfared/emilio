package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/util/errors"
)

type Options struct {
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

func ParseOptions() Options {
	o := Options{}
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
		logrus.WithError(err).Fatal("Failed to parse command line arguments")
	}

	return o
}

func (o Options) Validate() error {
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
