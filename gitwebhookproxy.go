package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/namsral/flag"
	"github.com/stakater/GitWebhookProxy/pkg/proxy"
)

var (
	flagSet       = flag.NewFlagSetWithEnvPrefix(os.Args[0], "GWP", 0)
	listenAddress = flagSet.String("listen", ":8080", "Address on which the proxy listens.")
	upstreamURL   = flagSet.String("upstreamURL", "", "URL to which the proxy requests will be forwarded") // Removed (required)
	upstreamURLs  = flagSet.String("upstreamURLs", "", "Comma-Separated String List of additional upstream URLs")
	secret        = flagSet.String("secret", "", "Secret of the Webhook API. If not set validation is not made.")
	provider      = flagSet.String("provider", "github", "Git Provider which generates the Webhook")
	allowedPaths  = flagSet.String("allowedPaths", "", "Comma-Separated String List of allowed paths")
	ignoredUsers  = flagSet.String("ignoredUsers", "", "Comma-Separated String List of users to ignore while proxying Webhook request")
	allowedUsers  = flagSet.String("allowedUser", "", "Comma-Separated String List of users to allow while proxying Webhook request")
)

func validateRequiredFlags() {
	isValid := true
	trimmedUpstreamURL := strings.TrimSpace(*upstreamURL)
	trimmedUpstreamURLs := strings.TrimSpace(*upstreamURLs)

	if len(trimmedUpstreamURL) == 0 && len(trimmedUpstreamURLs) == 0 {
		log.Println("Required flag 'upstreamURL' or 'upstreamURLs' must be specified")
		isValid = false
	}

	if isValid && len(trimmedUpstreamURL) > 0 {
		if !strings.HasPrefix(trimmedUpstreamURL, "http://") && !strings.HasPrefix(trimmedUpstreamURL, "https://") {
			log.Printf("Invalid URL format for 'upstreamURL': %s. URL must start with http:// or https://", trimmedUpstreamURL)
			isValid = false
		}
	}

	if isValid && len(trimmedUpstreamURLs) > 0 {
		urls := strings.Split(trimmedUpstreamURLs, ",")
		if len(urls) == 0 && len(trimmedUpstreamURL) == 0 { // This case should be caught by the first check, but good for safety
			log.Println("Required flag 'upstreamURLs' must contain at least one URL if 'upstreamURL' is not set")
			isValid = false
		}
		for _, url := range urls {
			trimmedSingleURL := strings.TrimSpace(url)
			if len(trimmedSingleURL) == 0 { // Allow empty strings from splitting, but they won't be added later
				continue
			}
			if !strings.HasPrefix(trimmedSingleURL, "http://") && !strings.HasPrefix(trimmedSingleURL, "https://") {
				log.Printf("Invalid URL format in 'upstreamURLs': %s. URL must start with http:// or https://", trimmedSingleURL)
				isValid = false
				break // Stop validation on first invalid URL in the list
			}
		}
	}

	if !isValid {
		fmt.Println("")
		//TODO: Usage not working as expected in flagSet
		flagSet.Usage()
		fmt.Println("")

		panic("See Flag Usage")
	}
}

func main() {
	flagSet.Parse(os.Args[1:])
	validateRequiredFlags()
	lowerProvider := strings.ToLower(*provider)

	// Split Comma-Separated list into an array
	allowedPathsArray := []string{}
	if len(*allowedPaths) > 0 {
		allowedPathsArray = strings.Split(*allowedPaths, ",")
	}

	// Split Comma-Separated list into an array
	ignoredUsersArray := []string{}
	if len(*ignoredUsers) > 0 {
		ignoredUsersArray = strings.Split(*ignoredUsers, ",")
	}

	log.Printf("Stakater Git WebHook Proxy started with provider '%s'\n", lowerProvider)

	allUpstreamURLs := []string{}
	seenURLs := make(map[string]bool)

	trimmedUpstreamURL := strings.TrimSpace(*upstreamURL)
	if len(trimmedUpstreamURL) > 0 {
		// Validation for individual URL format already happened in validateRequiredFlags
		if !seenURLs[trimmedUpstreamURL] {
			allUpstreamURLs = append(allUpstreamURLs, trimmedUpstreamURL)
			seenURLs[trimmedUpstreamURL] = true
		}
	}

	trimmedUpstreamURLs := strings.TrimSpace(*upstreamURLs)
	if len(trimmedUpstreamURLs) > 0 {
		urls := strings.Split(trimmedUpstreamURLs, ",")
		for _, urlStr := range urls {
			currentURL := strings.TrimSpace(urlStr)
			if len(currentURL) > 0 {
				// Validation for individual URL format already happened in validateRequiredFlags
				if !seenURLs[currentURL] {
					allUpstreamURLs = append(allUpstreamURLs, currentURL)
					seenURLs[currentURL] = true
				}
			}
		}
	}

	log.Printf("Consolidated upstream URLs: %v", allUpstreamURLs)

	p, err := proxy.NewProxy(allUpstreamURLs, allowedPathsArray, lowerProvider, *secret, ignoredUsersArray)
	if err != nil {
		log.Fatal(err)
	}

	if err := p.Run(*listenAddress); err != nil {
		log.Fatal(err)
	}

}
