package main

import (
	"context"
	"log"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/amirrmonfared/emilio/pkg/config"
	"github.com/amirrmonfared/emilio/pkg/imapclient"
	"github.com/amirrmonfared/emilio/pkg/openai"
	"github.com/amirrmonfared/emilio/pkg/processor"
)

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	logger := logrus.WithField("component", "fetcher")

	options := config.ParseOptions()
	if err := options.Validate(); err != nil {
		logger.WithError(err).Fatal("Invalid options")
	}

	ctx := context.Background()

	llm, err := openai.NewClient(options.APIKey)
	if err != nil {
		log.Fatalf("Failed to initialize OpenAI: %v", err)
	}

	processor := processor.New(logger, llm)

	if err := imapclient.ConnectAndProcessEmails(ctx, options, processor); err != nil {
		logger.WithError(err).Fatal("Failed to process emails")
	}
}
