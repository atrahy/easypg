package main

import (
	"fmt"
	"log"
	"os"

	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	pgUrlString = "postgres://local_user@localhost:5432/local_db"
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
	db, err := sql.Connect(pgUrlString)
	if err != nil {
		fmt.Printf("Debug: Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	p := tea.NewProgram(tui.NewModel(db))

	if _, err = p.Run(); err != nil {
		fmt.Printf("Tea error: %v\n", err)
		os.Exit(1)
	}
}
