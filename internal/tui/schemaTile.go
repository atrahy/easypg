package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type schemaTile struct {
	List list.Model
}

type item struct {
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.title }
func (i item) FilterValue() string { return i.title }

func newSchemaTile() *schemaTile {
	// columns := []table.Column{
	// 	{Title: "Name", Width: 20},
	// }

	// rows := []table.Row{}

	// t := table.New(
	// 	table.WithColumns(columns),
	// 	table.WithRows(rows),
	// 	// table.WithFocused(true),
	// 	table.WithHeight(7),
	// )

	// s := table.DefaultStyles()

	// s.Header = s.Header.
	// 	// BorderStyle(lipgloss.NormalBorder()).
	// 	Border(lipgloss.NormalBorder(), true).
	// 	BorderForeground(lipgloss.Color("240")).
	// 	Bold(false)

	// s.Selected = s.Selected.
	// 	Foreground(lipgloss.Color("229")).
	// 	Background(lipgloss.Color("57")).
	// 	Bold(false)
	// t.SetStyles(s)

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	// l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)

	return &schemaTile{l}
}

func (n *schemaTile) SetItems(rows []string) tea.Cmd {
	var items []list.Item

	for _, row := range rows {
		items = append(items, item{title: row})
	}

	return n.List.SetItems(items)
}

func (n *schemaTile) Update(msg tea.Msg) tea.Cmd {
	list, cmd := n.List.Update(msg)

	n.List = list
	return cmd
}

func (n *schemaTile) View() string {
	return n.List.View()
}

func (n *schemaTile) GetSelectedItemName() string {
	return n.List.SelectedItem().(item).title
}

func (n *schemaTile) Cursor() int {
	return n.List.Cursor()
}

func (n *schemaTile) SetSize(width, height int) {
	n.List.SetSize(width, height)
}
