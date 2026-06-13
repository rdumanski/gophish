package main

/*
gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/alecthomas/kingpin/v2"

	"github.com/rdumanski/gophish/ai"
	"github.com/rdumanski/gophish/config"
	"github.com/rdumanski/gophish/controllers"
	"github.com/rdumanski/gophish/dialer"
	"github.com/rdumanski/gophish/imap"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/middleware"
	"github.com/rdumanski/gophish/models"
	"github.com/rdumanski/gophish/webhook"
)

// buildAIGenerator constructs the configured AI generator. Returns
// (nil, nil) when the configured provider is unrecognised — callers
// treat that as "AI off" without aborting startup.
func buildAIGenerator(cfg config.AIConfig) (ai.Generator, error) {
	switch cfg.Provider {
	case "anthropic", "":
		// Fall back to the ANTHROPIC_API_KEY environment variable when the
		// config omits the key — keeps the secret out of config.json (and out
		// of source control / backups), the standard way to handle API keys.
		apiKey := cfg.Anthropic.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		return ai.NewAnthropic(ai.AnthropicConfig{
			APIKey:    apiKey,
			Model:     cfg.Anthropic.Model,
			MaxTokens: cfg.Anthropic.MaxTokens,
		})
	default:
		log.Errorf("ai: unknown provider %q in config", cfg.Provider)
		return nil, nil
	}
}

const (
	modeAll   string = "all"
	modeAdmin string = "admin"
	modePhish string = "phish"
)

var (
	configPath    = kingpin.Flag("config", "Location of config.json.").Default("./config.json").String()
	disableMailer = kingpin.Flag("disable-mailer", "Disable the mailer (for use with multi-system deployments)").Bool()
	mode          = kingpin.Flag("mode", fmt.Sprintf("Run the binary in one of the modes (%s, %s or %s)", modeAll, modeAdmin, modePhish)).
			Default("all").Enum(modeAll, modeAdmin, modePhish)
)

func main() {
	// Load the version

	version, err := os.ReadFile("./VERSION")
	if err != nil {
		log.Fatal(err)
	}
	kingpin.Version(string(version))

	// Parse the CLI flags and load the config
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.Parse()

	// Load the config
	conf, err := config.LoadConfig(*configPath)
	// Just warn if a contact address hasn't been configured
	if err != nil {
		log.Fatal(err)
	}
	if conf.ContactAddress == "" {
		log.Warnf("No contact address has been configured.")
		log.Warnf("Please consider adding a contact_address entry in your config.json")
	}
	config.Version = string(version)

	// Configure our various upstream clients to make sure that we restrict
	// outbound connections as needed.
	dialer.SetAllowedHosts(conf.AdminConf.AllowedInternalHosts)
	webhook.SetTransport(&http.Transport{
		DialContext: dialer.Dialer().DialContext,
	})

	err = log.Setup(conf.Logging)
	if err != nil {
		log.Fatal(err)
	}

	// Provide the option to disable the built-in mailer
	// Setup the global variables and settings
	err = models.Setup(conf)
	if err != nil {
		log.Fatal(err)
	}

	// Phase 7c.2: seed the phish_filter table from config.json on
	// first run so existing deployments don't lose their values.
	// After this point DB is the authoritative source; subsequent
	// edits via the Settings UI overwrite the seeded values.
	if err := models.SeedPhishFilterFromConfig(conf.PhishConf.SandboxFilter); err != nil {
		log.Errorf("phish_filter: seed-from-config failed: %s", err)
	}
	// Populate the in-process matcher cache so the first campaign
	// summary render after startup applies the policy. Errors here
	// (e.g. transient DB issue) just leave the cache nil — filter
	// effectively off until the next read succeeds.
	if _, err := models.GetPhishFilter(); err != nil {
		log.Errorf("phish_filter: initial load failed: %s", err)
	}

	// Unlock any maillogs that may have been locked for processing
	// when Gophish was last shutdown.
	err = models.UnlockAllMailLogs()
	if err != nil {
		log.Fatal(err)
	}

	// Create our servers
	adminOptions := []controllers.AdminServerOption{}
	if *disableMailer {
		adminOptions = append(adminOptions, controllers.WithWorker(nil))
	}
	if conf.AI.Enabled {
		gen, err := buildAIGenerator(conf.AI)
		if err != nil {
			log.Errorf("ai: features disabled — failed to construct generator: %s", err)
		} else if gen != nil {
			adminOptions = append(adminOptions, controllers.WithAIGenerator(gen))
		}
	}
	adminConfig := conf.AdminConf
	adminServer := controllers.NewAdminServer(adminConfig, adminOptions...)
	middleware.Store.Options.Secure = adminConfig.UseTLS

	phishConfig := conf.PhishConf
	phishServer := controllers.NewPhishingServer(phishConfig)

	imapMonitor := imap.NewMonitor()
	if *mode == "admin" || *mode == "all" {
		go adminServer.Start()
		go imapMonitor.Start()
	}
	if *mode == "phish" || *mode == "all" {
		go phishServer.Start()
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Info("CTRL+C Received... Gracefully shutting down servers")
	if *mode == modeAdmin || *mode == modeAll {
		adminServer.Shutdown()
		imapMonitor.Shutdown()
	}
	if *mode == modePhish || *mode == modeAll {
		phishServer.Shutdown()
	}

}
