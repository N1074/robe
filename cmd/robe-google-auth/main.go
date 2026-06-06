package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	calendaradapter "github.com/N1074/robe/internal/adapters/calendar"
	"github.com/N1074/robe/internal/config"
)

func main() {
	cfg := config.Load()

	if cfg.CalendarCredentialsFile == "" {
		log.Fatal("CALENDAR_CREDENTIALS_FILE is required")
	}
	if cfg.CalendarTokenFile == "" {
		log.Fatal("CALENDAR_TOKEN_FILE is required")
	}

	url, err := calendaradapter.AuthURL(cfg.CalendarCredentialsFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Open this URL, approve Calendar access, then paste the authorization code:")
	fmt.Println(url)
	fmt.Print("Code: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}

	if err := calendaradapter.ExchangeCode(context.Background(), cfg.CalendarCredentialsFile, cfg.CalendarTokenFile, strings.TrimSpace(code)); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Token saved to " + cfg.CalendarTokenFile)
}
