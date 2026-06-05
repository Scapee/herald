// Package providers registers LLM provider implementations with the tau provider registry.
package providers

import (
	"context"
	"fmt"
	"maps"
	"net/http"

	"github.com/tailored-agentic-units/protocol"
	protostreaming "github.com/tailored-agentic-units/protocol/streaming"
	"github.com/tailored-agentic-units/provider"
	"github.com/tailored-agentic-units/provider/streaming"
	tauconfig "github.com/tailored-agentic-units/protocol/config"
)

// RegisterOpenAI registers the OpenAI provider with the global provider registry.
func RegisterOpenAI() {
	provider.Register("openai", newOpenAI)
}

func newOpenAI(c *tauconfig.ProviderConfig) (provider.Provider, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &openAIProvider{
		BaseProvider: provider.NewBaseProvider(c.Name, baseURL),
		options:      c.Options,
		stream:       streaming.NewSSEReader(),
	}, nil
}

type openAIProvider struct {
	*provider.BaseProvider
	options map[string]any
	stream  protostreaming.StreamReader
}

func (p *openAIProvider) Endpoint(proto protocol.Protocol) (string, error) {
	endpoints := map[protocol.Protocol]string{
		protocol.Chat:       "/chat/completions",
		protocol.Vision:     "/chat/completions",
		protocol.Tools:      "/chat/completions",
		protocol.Embeddings: "/embeddings",
	}

	endpoint, exists := endpoints[proto]
	if !exists {
		return "", fmt.Errorf("protocol %s not supported by OpenAI", proto)
	}

	return fmt.Sprintf("%s%s", p.BaseURL(), endpoint), nil
}

func (p *openAIProvider) Stream() protostreaming.StreamReader {
	return p.stream
}

func (p *openAIProvider) PrepareRequest(ctx context.Context, proto protocol.Protocol, body []byte, headers map[string]string) (*provider.Request, error) {
	endpoint, err := p.Endpoint(proto)
	if err != nil {
		return nil, err
	}

	return &provider.Request{
		URL:     endpoint,
		Headers: headers,
		Body:    body,
	}, nil
}

func (p *openAIProvider) PrepareStreamRequest(ctx context.Context, proto protocol.Protocol, body []byte, headers map[string]string) (*provider.Request, error) {
	endpoint, err := p.Endpoint(proto)
	if err != nil {
		return nil, err
	}

	streamHeaders := make(map[string]string)
	maps.Copy(streamHeaders, headers)
	streamHeaders["Accept"] = protostreaming.SSEMedia
	streamHeaders["Cache-Control"] = "no-cache"

	return &provider.Request{
		URL:     endpoint,
		Headers: streamHeaders,
		Body:    body,
	}, nil
}

func (p *openAIProvider) SetHeaders(ctx context.Context, req *http.Request) error {
	if key, ok := p.options["api_key"].(string); ok && key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return nil
}
