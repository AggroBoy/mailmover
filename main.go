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
)

var buildSHA = "dev"

func main() {
	log.Printf("MailMover %s startup", buildSHA)
	os.Exit(run())
}

func run() int {
	// Get config from environment variables
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
	idleCleanup, idleUpdates, idleEnd, err := idleSetup(imapServer, tlsConfig, fromFolder, username, password, idleStop)
	if err != nil {
		log.Printf("setting-up IDLE connection: %v", err)
		return 1
	}
	defer idleCleanup()
	defer func() {
		close(idleStop)
		<-idleEnd
	}()

	// Command setup
	comCon, err := client.DialTLS(imapServer, tlsConfig)
	if err != nil {
		log.Printf("connecting command connection: %v", err)
		return 1
	}
	defer comCon.Logout()
	if err := comCon.Login(username, password); err != nil {
		log.Printf("logging-in command connection: %v", err)
		return 1
	}

	if err := processFolder(fromFolder, toFolder, comCon); err != nil {
		log.Printf("processing %v folder: %v", fromFolder, err)
		return 1
	}

	for {
		select {
		case <-idleUpdates:
			log.Printf("Received IDLE update for %s", fromFolder)
			if err := processFolder(fromFolder, toFolder, comCon); err != nil {
				log.Printf("processing %v folder: %v", fromFolder, err)
				return 1
			}
		case err := <-idleEnd:
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
	idleEnd := make(chan error, 1)
	go func() {
		idleEnd <- idleCon.Idle(idleStop, nil)
	}()
	idleCon.Updates = idleUpdates

	cleanup := func() {
		if err := idleCon.Logout(); err != nil {
			log.Printf("logging out of IDLE connection: %v", err)
		}
	}

	return cleanup, idleUpdates, idleEnd, nil
}

func processFolder(fromFolder string, toFolder string, c *client.Client) error {
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

	return "", fmt.Errorf("setting %s not found in category %s", name, category)
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
