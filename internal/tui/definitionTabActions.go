package tui

import (
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/objectsPane"
)

type schemaList struct {
	schemas []sql.Namespace
}

type tablesList struct {
	tables []sql.Table
}

type functionsList struct {
	functions []sql.Function
}

type tableAttr struct {
	tableAttr *sql.TableAttr
	objType   string
}

type functionDef struct {
	def string
}

// fetchErrMsg surfaces a failed query into the UI instead of crashing the app.
type fetchErrMsg struct {
	err error
}

func (t *definitionTabModel) fetchNamespaces() tea.Msg {
	result, err := t.db.QueryNamespaces()
	if err != nil {
		log.Printf("Failed to query init schemas: %v\n", err)
		return fetchErrMsg{err}
	}

	return schemaList{schemas: result}
}

func (t *definitionTabModel) fetchTables(schema string) tea.Cmd {
	return func() tea.Msg {
		result, err := t.db.QueryTablesForSchema(schema)
		if err != nil {
			log.Printf("Failed to query tables for schema: %s: %v\n", schema, err)
			return fetchErrMsg{err}
		}

		return tablesList{tables: result}
	}
}

func (t *definitionTabModel) fetchFunctions(schema string) tea.Cmd {
	return func() tea.Msg {
		result, err := t.db.QueryFunctionsForSchema(schema)
		if err != nil {
			log.Printf("Failed to query functions for schema: %s: %v\n", schema, err)
			return fetchErrMsg{err}
		}

		return functionsList{functions: result}
	}
}

func (t *definitionTabModel) fetchTableAttr(sel objectsPane.Selection) tea.Cmd {
	return func() tea.Msg {
		result, err := t.db.QueryTableAttr(sql.Table{
			OID:    sel.OID,
			Schema: sel.Schema,
			Name:   sel.Name,
			Type:   sel.Kind,
		})
		if err != nil {
			log.Printf("Failed to query table attr for oid: %s: %v\n", sel.OID, err)
			return fetchErrMsg{err}
		}

		return tableAttr{tableAttr: result, objType: sel.Kind}
	}
}

func (t *definitionTabModel) fetchFunctionDef(oid string) tea.Cmd {
	return func() tea.Msg {
		def, err := t.db.QueryFunctionDef(oid)
		if err != nil {
			log.Printf("Failed to query function def for oid: %s: %v\n", oid, err)
			return fetchErrMsg{err}
		}

		return functionDef{def: def}
	}
}
