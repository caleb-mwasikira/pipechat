// Command chat is the Pipechat terminal client. It needs no configuration: on
// first run it asks you to /connect to a chat server and remembers it.
//
// How it starts:
//
//  1. read the command line (both options are optional)
//  2. open the small local file that remembers your server and last handle
//  3. build the screen, a Bubble Tea program
//  4. start connecting to the server, if we know one; what happens there is sent to the screen
//  5. run the screen until you quit
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client"
)

func main() {
	// 1. Command line.
	defaultStorePath, err := client.DefaultStorePath()
	if err != nil {
		log.Fatalf("config directory: %v", err)
	}
	serverURL := flag.String("server", "", "chat server to use, for example https://chat.example.com (default: the one from last time)")
	storePath := flag.String("store", defaultStorePath, "file that remembers your server and last handle")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: chat [--server URL] [--store FILE]")
		os.Exit(2)
	}

	// 2. Local store.
	store, err := client.LoadStore(*storePath)
	if err != nil {
		log.Fatalf("store %s: %v", *storePath, err)
	}

	// 3. Screen.
	screen := client.NewModel(store, *serverURL)
	program := tea.NewProgram(screen, tea.WithAltScreen())

	// 4. Server connection. program.Send is how it reaches the screen.
	screen.Connect(program.Send)

	// 5. Run.
	if _, err := program.Run(); err != nil {
		log.Fatalf("screen: %v", err)
	}
}
