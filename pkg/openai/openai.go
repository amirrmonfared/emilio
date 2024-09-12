package openai

import (
	"context"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type OpenAI struct {
	llm llms.Model
}

// NewClient initializes a new OpenAI client with the provided API key.
func NewClient(apiKey string) (OpenAI, error) {
	llm, err := openai.New(openai.WithToken(apiKey))
	if err != nil {
		return OpenAI{}, err
	}
	return OpenAI{llm: llm}, nil
}

// Call sends a prompt to the OpenAI model and returns the response.
func (o OpenAI) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	response, err := o.llm.Call(ctx, prompt, options...)
	if err != nil {
		return "", err
	}
	return response, nil
}

// WithTemperature is a utility function to set the temperature for OpenAI calls.
func WithTemperature(temp float64) llms.CallOption {
	return llms.WithTemperature(temp)
}
