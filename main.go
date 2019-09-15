// Copyright 2018 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Wrapper script for running a Cheesy Arena remote display that could be either on the field network or remote.

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	displayIdFilePath = "/boot/display_id"
	localServerUrl    = "http://10.0.100.5:8080/display?displayId="
	remoteServerUrl   = "https://cheesyarena.com/display?displayId="
	httpTimeout       = 5 * time.Second
	pollPeriod        = 5 * time.Second
)

// Main entry point for the application.
func main() {
	// Log both to file and to stdout.
	logFile, err := os.OpenFile("cheesy-arena-rpi.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	var displayId, serverUrl string
	for {
		// Loop until either the local or remote path to the Cheesy Arena server works.
		if displayId = tryGetDisplayId(localServerUrl); displayId != "" {
			serverUrl = localServerUrl
			break
		}
		if displayId = tryGetDisplayId(remoteServerUrl); displayId != "" {
			serverUrl = remoteServerUrl
			break
		}

		log.Printf("Unsuccessful at connecting; waiting %v before trying again.", pollPeriod)
		time.Sleep(pollPeriod)
	}

	// Try to read the stored display ID if it exists.
	if displayIdBytes, _ := ioutil.ReadFile(displayIdFilePath); len(displayIdBytes) > 0 {
		displayId = strings.TrimSpace(string(displayIdBytes))
		log.Printf("Using existing stored display ID '%s'.", displayId)
	} else {
		log.Printf("Using new display ID '%s'.", displayId)
	}

	var browserCommand *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		browserCommand = exec.Command("open", "-a", "Google Chrome", "--args", "--start-fullscreen",
			fmt.Sprintf("--app=%s%s", serverUrl, displayId))
	case "linux":
		browserCommand = exec.Command("chromium-browser", "--start-fullscreen",
			fmt.Sprintf("--app=%s%s", serverUrl, displayId))
	default:
		log.Fatalf("Don't know how to launch browser for unsupported operating system '%s'.", runtime.GOOS)
	}

	// Shell out to launch a browser window for the display.
	if err := browserCommand.Run(); err != nil {
		log.Fatalln(err)
	}
}

// Attempts to connect to the given Cheesy Arena server endpoint. Returns the suggested new display ID from the redirect
// response if successful, or the empty string on failure.
func tryGetDisplayId(url string) string {
	log.Printf("Checking %s for a connection to the Cheesy Arena server.", url)
	httpClient := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return ""
	}

	// Check for the expected redirect response and extract the suggested display ID.
	if response.StatusCode == 302 {
		displayIdRe := regexp.MustCompile("displayId=(\\d+)")
		if matches := displayIdRe.FindStringSubmatch(response.Header.Get("Location")); len(matches) > 0 {
			log.Printf("Successfully connected to %s with suggested display ID '%s'.", url, matches[1])
			return matches[1]
		}
	}
	return ""
}
