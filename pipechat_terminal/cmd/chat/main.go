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
	"pipechat/internal/client/update"
)

func main() {
	// 1. Command line.
	defaultStorePath, err := client.DefaultStorePath()
	if err != nil {
		log.Fatalf("config directory: %v", err)
	}
	serverURL := flag.String("server", "", "chat server to use, for example https://chat.example.com (default: the one from last time)")
	storePath := flag.String("store", defaultStorePath, "file that remembers your server and last handle")
	showVersion := flag.Bool("version", false, "print the version of this program and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("pipechat " + update.Current)
		return
	}
	update.Cleanup() // what an earlier update left beside the program
	if flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: chat [--server URL] [--store FILE] [--version]")
		os.Exit(2)
	}

	// 2. Local store.
	store, err := client.LoadStore(*storePath)
	if err != nil {
		log.Fatalf("store %s: %v", *storePath, err)
	}

	// 3. Screen.
	screen := client.NewModel(store, *serverURL)
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if store.MouseOn {
		options = append(options, tea.WithMouseCellMotion()) // the mouse wheel scrolls (/mouse on); otherwise text can be selected and copied
	}
	program := tea.NewProgram(screen, options...)

	// 4. Server connection. program.Send is how it reaches the screen.
	screen.Connect(program.Send)

	// 5. Run.
	if _, err := program.Run(); err != nil {
		log.Fatalf("screen: %v", err)
	}

	// 6. After an update, start the new program in place of this one.
	if exe := screen.RestartExe(); exe != "" {
		fmt.Println("Updated. Starting the new version...")
		if err := update.Restart(exe, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "updated, but could not start the new version (%v); run %s again\n", err, exe)
			os.Exit(1)
		}
	}
}
