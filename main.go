package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/atrahy/easypg/internal/config"
	"github.com/atrahy/easypg/internal/tui"
)

func init() {
	// Create a log file
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// Set log output to the file
	log.SetOutput(file)
}

func main() {
	var requested string

	flag.StringVar(&requested, "connection", "", "name of the connection to start on")
	flag.StringVar(&requested, "c", "", "shorthand for -connection")
	flag.Parse()

	if err := run(requested); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run wires the config to the TUI. Nothing connects here: the root model owns
// the connection, since it is the one that can swap it at runtime, and a
// database that does not answer must produce an error on screen rather than a
// program that never draws anything.
func run(requested string) error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	conns, err := config.LoadConnections(dir)
	if err != nil {
		return err
	}

	// A nil target with no error is the ordinary case: nothing was asked for on
	// the command line, so the screen simply opens with its cursor at the top.
	target, err := conns.Resolve(requested)
	if err != nil {
		return err
	}

	final, err := tea.NewProgram(tui.NewModel(dir, cfg, conns, target)).Run()
	if err != nil {
		return fmt.Errorf("tea: %w", err)
	}

	if model, ok := final.(tui.Model); ok {
		model.Close()
	}

	return nil
}
