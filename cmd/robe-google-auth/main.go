package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	calendaradapter "github.com/N1074/robe/internal/adapters/calendar"
	gmailadapter "github.com/N1074/robe/internal/adapters/gmail"
	"github.com/N1074/robe/internal/config"
)

func main() {
	cfg := config.Load()

	target := strings.TrimSpace(os.Getenv("GOOGLE_AUTH_TARGET"))
	if target == "" {
		target = "calendar"
	}

	credentialsFile, tokenFile, label := googleAuthFiles(cfg, target)
	if credentialsFile == "" {
		log.Fatalf("%s credentials file is required", label)
	}
	if tokenFile == "" {
		log.Fatalf("%s token file is required", label)
	}

	url, err := googleAuthURL(target, credentialsFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Open this URL, approve %s access, then paste the authorization code:\n", label)
	fmt.Println(url)
	fmt.Print("Code: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}

	if err := googleExchangeCode(context.Background(), target, credentialsFile, tokenFile, strings.TrimSpace(code)); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Token saved to " + tokenFile)
}

func googleAuthFiles(cfg config.Config, target string) (string, string, string) {
	switch target {
	case "calendar":
		return cfg.CalendarCredentialsFile, cfg.CalendarTokenFile, "Calendar"
	case "gmail":
		return cfg.GmailCredentialsFile, cfg.GmailTokenFile, "Gmail modify"
	default:
		log.Fatalf("unsupported GOOGLE_AUTH_TARGET %q; use calendar or gmail", target)
		return "", "", ""
	}
}

func googleAuthURL(target string, credentialsFile string) (string, error) {
	switch target {
	case "calendar":
		return calendaradapter.AuthURL(credentialsFile)
	case "gmail":
		return gmailadapter.AuthURL(credentialsFile)
	default:
		return "", fmt.Errorf("unsupported GOOGLE_AUTH_TARGET %q", target)
	}
}

func googleExchangeCode(ctx context.Context, target string, credentialsFile string, tokenFile string, code string) error {
	switch target {
	case "calendar":
		return calendaradapter.ExchangeCode(ctx, credentialsFile, tokenFile, code)
	case "gmail":
		return gmailadapter.ExchangeCode(ctx, credentialsFile, tokenFile, code)
	default:
		return fmt.Errorf("unsupported GOOGLE_AUTH_TARGET %q", target)
	}
}
