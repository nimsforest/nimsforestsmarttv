package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	smarttv "github.com/nimsforest/nimsforestsmarttv"
)

var scanner *bufio.Scanner

func main() {
	ctx := context.Background()
	scanner = bufio.NewScanner(os.Stdin)

	fmt.Println("Smart TV Renderer")
	fmt.Println("=================")
	fmt.Println()

	// Discover TVs
	tvs := discoverTVs(ctx)
	if len(tvs) == 0 {
		fmt.Println("No TVs found. Use /discover to scan again.")
	}

	// Select TV
	var selectedTV *smarttv.TV
	if len(tvs) == 1 {
		selectedTV = &tvs[0]
		fmt.Printf("Selected: %s\n", selectedTV)
	} else if len(tvs) > 1 {
		selectedTV = selectTV(tvs)
	}

	// Create renderer
	renderer, err := smarttv.NewRenderer()
	if err != nil {
		fmt.Printf("Error creating renderer: %v\n", err)
		os.Exit(1)
	}
	defer renderer.Close()

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  /discover  - Scan for TVs")
	fmt.Println("  /select    - Select a different TV")
	fmt.Println("  /stop      - Stop displaying")
	fmt.Println("  /quit      - Exit")
	fmt.Println()
	fmt.Println("Type any text to display it on the TV.")
	fmt.Println()

	// Interactive loop
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case input == "/quit" || input == "/exit" || input == "/q":
			fmt.Println("Goodbye!")
			return

		case input == "/discover":
			tvs = discoverTVs(ctx)
			if len(tvs) == 0 {
				fmt.Println("No TVs found.")
			}

		case input == "/select":
			if len(tvs) == 0 {
				fmt.Println("No TVs available. Use /discover first.")
				continue
			}
			selectedTV = selectTV(tvs)

		case input == "/stop":
			if selectedTV == nil {
				fmt.Println("No TV selected. Use /select first.")
				continue
			}
			if err := renderer.Stop(ctx, selectedTV); err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Stopped.")
			}

		case strings.HasPrefix(input, "/"):
			fmt.Printf("Unknown command: %s\n", input)

		default:
			// Display text on TV
			if selectedTV == nil {
				fmt.Println("No TV selected. Use /select first.")
				continue
			}

			fmt.Printf("Sending to %s...\n", selectedTV.Name)
			if err := renderer.DisplayText(ctx, selectedTV, input); err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Done!")
			}
		}
	}
}

func discoverTVs(ctx context.Context) []smarttv.TV {
	fmt.Println("Discovering TVs...")
	tvs, err := smarttv.Discover(ctx, 5*time.Second)
	if err != nil {
		fmt.Printf("Discovery error: %v\n", err)
		return nil
	}

	fmt.Printf("Found %d TV(s):\n", len(tvs))
	for i, tv := range tvs {
		fmt.Printf("  [%d] %s\n", i+1, tv.String())
	}

	return tvs
}

func selectTV(tvs []smarttv.TV) *smarttv.TV {
	if len(tvs) == 0 {
		return nil
	}

	if len(tvs) == 1 {
		fmt.Printf("Selected: %s\n", tvs[0].String())
		return &tvs[0]
	}

	fmt.Println()
	fmt.Println("Select a TV:")
	for i, tv := range tvs {
		fmt.Printf("  [%d] %s\n", i+1, tv.String())
	}

	for {
		fmt.Print("Enter number: ")
		if !scanner.Scan() {
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(tvs) {
			fmt.Printf("Please enter a number between 1 and %d\n", len(tvs))
			continue
		}

		selected := &tvs[num-1]
		fmt.Printf("Selected: %s\n", selected.String())
		return selected
	}
}
