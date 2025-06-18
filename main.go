package main

import (
	"crypto/tls"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var buildSHA = "dev"

func main() {
	log.Printf("MailMover %s startup", buildSHA)
	os.Exit(run())
}

func run() int {
	// Get config
	username := getMandatorySecretValue("IMAP_USERNAME")
	password := getMandatorySecretValue("IMAP_PASSWORD")
	imapServer := getMandatoryConfigValue("IMAP_SERVER")
	fromFolder := getMandatoryConfigValue("FROM_FOLDER")
	toFolder := getMandatoryConfigValue("TO_FOLDER")

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Add additional TLS configurations as needed
	}

	// IDLE setup
	idleStop := make(chan struct{})
	idleCleanup, idleUpdates, idleError, err := idleSetup(imapServer, tlsConfig, fromFolder, username, password, idleStop)
	if err != nil {
		log.Printf("setting-up IDLE connection: %v", err)
		return 1
	}
	defer idleCleanup()
	defer func() {
		close(idleStop)
		select {
		case <-idleError:
			// graceful exit path
		case <-time.After(2 * time.Second):
			_, _ = fmt.Fprintln(os.Stderr, "Timeout waiting for idle goroutine to exit")
		}
	}()

	if err := processFolder(imapServer, username, password, fromFolder, toFolder); err != nil {
		log.Printf("processing %v folder: %v", fromFolder, err)
	}

	for {
		select {
		case <-idleUpdates:
			log.Printf("Received IDLE update for %s", fromFolder)
			if err := processFolder(imapServer, username, password, fromFolder, toFolder); err != nil {
				log.Printf("processing %v folder: %v", fromFolder, err)
			}
		case err := <-idleError:
			if err != nil {
				log.Printf("IDLE error: %v", err)
				return 1
			}
		}
	}
}

func idleSetup(imapServer string, tlsConfig *tls.Config, fromFolder string, username string, password string, idleStop chan struct{}) (func(), chan client.Update, chan error, error) {
	idleCon, err := client.DialTLS(imapServer, tlsConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connecting: %w", err)
	}
	if err := idleCon.Login(username, password); err != nil {
		return nil, nil, nil, fmt.Errorf("logging-in: %w", err)
	}
	_, err = idleCon.Select(fromFolder, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("selecting %v folder: %w", fromFolder, err)
	}
	idleUpdates := make(chan client.Update)
	idleError := make(chan error, 1)
	go func() {
		idleError <- idleCon.Idle(idleStop, nil)
	}()
	idleCon.Updates = idleUpdates

	cleanup := func() {
		if err := idleCon.Logout(); err != nil {
			log.Printf("logging out of IDLE connection: %v", err)
		}
	}

	return cleanup, idleUpdates, idleError, nil
}

func processFolder(imapServer string, username string, password string, fromFolder string, toFolder string) error {
	// Create a command connection
	// this could probably be cached, but long-running sockets require management; keeping it for just one invocation
	// is the simpler, less error-prone approach.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Add additional TLS configurations as needed
	}
	c, err := client.DialTLS(imapServer, tlsConfig)
	if err != nil {
		return fmt.Errorf("connecting command connection: %w", err)
	}
	defer func(c *client.Client) {
		err := c.Logout()
		if err != nil {
			log.Printf("logging out of command connection: %v", err)
		}
	}(c)
	if err := c.Login(username, password); err != nil {
		return fmt.Errorf("logging-in command connection: %w", err)
	}

	mbox, err := c.Select(fromFolder, false)
	if err != nil {
		return fmt.Errorf("selecting %v folder: %w", fromFolder, err)
	}

	if mbox.Messages > 0 {
		log.Printf("Processing %d messages", mbox.Messages)

		seqSet := new(imap.SeqSet)
		seqSet.AddRange(1, mbox.Messages)

		if err := c.Store(seqSet, imap.AddFlags, []interface{}{imap.SeenFlag}, nil); err != nil {
			return fmt.Errorf("marking %v messages as seen: %w", mbox.Messages, err)
		}

		if err := c.Move(seqSet, toFolder); err != nil {
			return fmt.Errorf("moving %v messages to %v: %w", mbox.Messages, toFolder, err)
		}
	}
	return nil
}

func resolveSetting(name string, category string) (string, error) {
	// 1. Look in environment
	if val := os.Getenv(name); val != "" {
		return val, nil
	}

	// 2. Look in config path
	path := filepath.Join("/etc/mailmover/", category, name)
	if val, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(val)), nil
	}

	return "", fmt.Errorf("%s %s not found (%s)", category, name, path)
}

func getMandatoryConfigValue(name string) string {
	value, err := resolveSetting(name, "config")
	if err != nil {
		log.Fatalf("%v", err)
	}

	return value
}

func getMandatorySecretValue(name string) string {
	value, err := resolveSetting(name, "secrets")
	if err != nil {
		log.Fatalf("%v", err)
	}

	return value
}
