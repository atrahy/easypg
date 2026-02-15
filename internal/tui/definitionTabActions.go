package tui

import (
	"log"
	"os"

	"github.com/atrahy/easypg/internal/sql"
	tea "github.com/charmbracelet/bubbletea"
)

type schemaList struct {
	schemas []sql.Namespace
}

type tablesList struct {
	tables []sql.Table
}

type tableAttr struct {
	tableAttr *sql.TableAttr
}

func (t *definitionTabModel) fetchNamespaces() tea.Msg {
	result, err := t.db.QueryNamespaces()
	if err != nil {
		log.Printf("Failed to query init schemas: %v\n", err)
		os.Exit(1)
	}

	return schemaList{schemas: result}
}

func (t *definitionTabModel) fetchTables(schema string) tea.Cmd {
	return func() tea.Msg {
		result, err := t.db.QueryTablesForSchema(schema)
		if err != nil {
			log.Printf("Failed to query tables for schema: %s: %v\n", schema, err)
			os.Exit(1)
		}

		return tablesList{tables: result}
	}
}

func (t *definitionTabModel) fetchTableAttr(tableOID string) tea.Cmd {
	return func() tea.Msg {
		result, err := t.db.QueryTableAttr(tableOID)
		if err != nil {
			log.Printf("Failed to query table attr for oid: %s: %v\n", tableOID, err)
			os.Exit(1)
		}

		return tableAttr{tableAttr: result}
	}
}
