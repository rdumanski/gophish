// Package sms provides the abstraction the worker uses to send SMS
// phishing messages. The Sender interface keeps the worker provider-
// agnostic; concrete implementations (Twilio in this phase, Vonage /
// AWS SNS / SignalWire later) plug in behind the same shape.
//
// Mirrors the design of the ai package: small interface, typed errors
// callers can map to HTTP status codes, factory keyed by a Provider
// string read from models.SMSProfile so adding a new provider is a
// single switch arm.
package sms
