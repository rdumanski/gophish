// Package ai provides provider-agnostic LLM integrations used by Gophish.
//
// The primary surface is the Generator interface, used for AI-assisted
// drafting of phishing-simulation email templates. Today the only
// implementation is the Anthropic-backed NewAnthropic, but the interface
// is designed to accept additional providers (OpenAI, local models)
// without changing call sites.
//
// All Generators are safe for concurrent use by multiple goroutines once
// constructed.
package ai
