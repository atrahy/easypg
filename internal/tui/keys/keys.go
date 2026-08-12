// Package keys is the single source of truth for the application's key
// bindings. Panes bind their behavior to it, and both the footer hint and the
// "?" overlay are generated from it, so a key can never be implemented in one
// place and documented (or forgotten) in another.
//
// Convention: letters are application commands, scrolling goes through j/k, the
// arrows, ctrl-prefixed paging and g/G. The bubbles defaults (bare f/b/u/d and
// space in the viewport, left/right for horizontal scrolling) are deliberately
// replaced, since those keys are needed for commands and pane navigation.
package keys

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type KeyMap struct {
	// Global
	Help        key.Binding
	Search      key.Binding
	Quit        key.Binding
	ForceQuit   key.Binding
	Cancel      key.Binding
	Refresh     key.Binding
	RefreshAll  key.Binding
	Zoom        key.Binding
	Copy        key.Binding
	Connections key.Binding

	// Connection screen and its wizard
	NewConn      key.Binding
	ForgetSecret key.Binding
	NextField    key.Binding
	PrevField    key.Binding
	TestConn     key.Binding
	SaveConn     key.Binding
	RevealSecret key.Binding
	StoreSecret  key.Binding

	// Focus
	NextPane    key.Binding
	PrevPane    key.Binding
	PaneSchema  key.Binding
	PaneObjects key.Binding
	PaneDetail  key.Binding

	// Internal tab strip of a pane
	NextTab key.Binding
	PrevTab key.Binding

	// Movement inside the focused pane
	Up           key.Binding
	Down         key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Top          key.Binding
	Bottom       key.Binding

	// Text views (SQL tile, help overlay)
	Wrap        key.Binding
	ScrollLeft  key.Binding
	ScrollRight key.Binding

	// Detail pane
	Inspector key.Binding

	// Search mode
	NextMatch    key.Binding
	PrevMatch    key.Binding
	AcceptSearch key.Binding
}

var Default = KeyMap{
	Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	// "/" does what fits the focused pane: filter a list, search a text view.
	// ShortHelp/FullHelp relabel it accordingly.
	Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter / search")),
	Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	ForceQuit:  key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel / close")),
	Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh pane")),
	RefreshAll: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh all")),
	Zoom:       key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "zoom pane")),
	Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy to clipboard")),
	// "c" opens the connection screen: a free letter, and the initial of what it
	// shows. It is listed under Global in "?", so it is learned like every other
	// key rather than from the documentation.
	Connections: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connections")),

	// The wizard's own keys. "n" doubles as NextMatch elsewhere, but the picker
	// holds no search matches, so the two never apply at the same time.
	NewConn: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new connection")),
	// The way out of a vault entry gone wrong, without opening the system's own
	// keychain UI to fix what this app put there.
	ForgetSecret: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forget password")),
	NextField: key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab", "next field")),
	PrevField: key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab", "previous field")),
	// The form's actions carry a function-key alias each. A terminal cannot
	// deliver cmd+… to a TUI (macOS terminals send no Super modifier outside the
	// Kitty keyboard protocol, which alacritty and Terminal.app do not implement),
	// so the portable second option is F-keys — which also gives ctrl+s a way out
	// on the terminals where it still means XOFF.
	TestConn: key.NewBinding(key.WithKeys("ctrl+t", "f2"), key.WithHelp("ctrl+t/f2", "test")),
	// Enter submits, as in any form: the fields are walked with tab, so the key
	// is free, and it is the one nobody has to be taught.
	SaveConn: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save & connect")),
	// A masked field one cannot unmask is a field one retypes on every typo.
	RevealSecret: key.NewBinding(key.WithKeys("ctrl+r", "f3"), key.WithHelp("ctrl+r/f3", "show password")),
	// Storing is the declared intent of auth = "keychain", but a shared or
	// temporary machine is a good enough reason to decline it once.
	StoreSecret: key.NewBinding(key.WithKeys("ctrl+s", "f4"), key.WithHelp("ctrl+s/f4", "remember")),

	// h/l and the arrows are aliases of tab/shift+tab: focus cycles through the
	// panes rather than moving geometrically, so every pane is reachable with
	// either hand and no move is ever a no-op.
	NextPane:    key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab/l/→", "next pane")),
	PrevPane:    key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab/h/←", "previous pane")),
	PaneSchema:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "focus schema")),
	PaneObjects: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "focus objects")),
	PaneDetail:  key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "focus detail")),

	NextTab: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next tab")),
	PrevTab: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "previous tab")),

	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half page up")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half page down")),
	PageUp:       key.NewBinding(key.WithKeys("ctrl+b", "pgup"), key.WithHelp("ctrl+b", "page up")),
	PageDown:     key.NewBinding(key.WithKeys("ctrl+f", "pgdown"), key.WithHelp("ctrl+f", "page down")),
	Top:          key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	Bottom:       key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),

	Wrap:        key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "toggle wrap")),
	ScrollLeft:  key.NewBinding(key.WithKeys("shift+left", "H"), key.WithHelp("shift+←/H", "scroll left")),
	ScrollRight: key.NewBinding(key.WithKeys("shift+right", "L"), key.WithHelp("shift+→/L", "scroll right")),

	Inspector: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "toggle inspector")),

	NextMatch:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
	PrevMatch:    key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "previous match")),
	AcceptSearch: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm search")),
}

// Context describes what currently has focus, so the generated help lists the
// keys that actually apply — and only those.
type Context struct {
	// Pane is the focused pane's display name, used as a help section title.
	Pane string
	// HasTabs tells whether the pane has an internal [ / ] tab strip.
	HasTabs bool
	// IsDetail enables the detail-only keys (inspector).
	IsDetail bool
	// IsText tells whether the active tile is a text view (SQL, help), which
	// adds wrapping and horizontal scrolling.
	IsText bool
	// IsList tells whether the active tile is a row list, which can be filtered.
	IsList bool
	// WrapOn and CanScrollX describe the state of a text tile. The hint line
	// doubles as its readout, since the tile itself draws none.
	WrapOn     bool
	CanScrollX bool
	// InspectorOpen is the detail pane's fold state, so the hint can say what
	// the key will do rather than that it toggles something.
	InspectorOpen bool
}

// Section is a titled group of bindings in the help overlay.
type Section struct {
	Title    string
	Bindings []key.Binding
}

// ShortHelp is the one-line hint under the status bar: what changes with the
// context, and only that.
//
// The keys that are the same everywhere — cycling panes, moving the cursor,
// quitting — are deliberately absent: they never vary, they are the first thing
// a user learns, and repeating them on every screen crowds out the entries that
// actually depend on where you are. "?" lists them, and the bar advertises "?".
func (k KeyMap) ShortHelp(ctx Context) []key.Binding {
	// "/" leads: it is the key one reaches for most, and its label is the one
	// that says what kind of pane this is (rows to filter, or text to search).
	bindings := []key.Binding{k.searchBinding(ctx)}

	if ctx.IsText {
		// The wrap entry doubles as the tile's state readout now that the tile
		// draws no status line of its own: "w: wrap off" says both what the key
		// does and where it currently stands.
		bindings = append(bindings, relabel(k.Wrap, wrapState(ctx.WrapOn)))

		if ctx.CanScrollX {
			bindings = append(bindings, k.ScrollHorizontalHint())
		}
	}

	// The inspector strip is the one pane element nothing on screen names, so the
	// hint spells out what "i" will do to it — it is inert on the SQL tab, which
	// has no strip, and absent from the line there.
	if ctx.IsDetail && ctx.IsList {
		bindings = append(bindings, relabel(k.Inspector, inspectorState(ctx.InspectorOpen)))
	}

	return bindings
}

func wrapState(on bool) string {
	if on {
		return "wrap on"
	}

	return "wrap off"
}

func inspectorState(open bool) string {
	if open {
		return "hide inspector"
	}

	return "show inspector"
}

// searchBinding relabels "/" with what it actually does here, so the footer and
// the help never promise a filter on a text view (or the reverse).
func (k KeyMap) searchBinding(ctx Context) key.Binding {
	if ctx.IsList {
		return key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter rows"))
	}

	return key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search text"))
}

// OverlayHelp is the hint line at the bottom of the "?" window: what the keys do
// *while it is open*. "enter" and "esc" are relabeled here — in the overlay they
// run the highlighted binding and close the window, they do not confirm or
// cancel a search — and they keep their keys, so this stays one declaration.
func (k KeyMap) OverlayHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithHelp("↑/↓ j/k", "move")),
		relabel(k.AcceptSearch, "run selected"),
		relabel(k.Search, "filter"),
		relabel(k.Cancel, "close"),
	}
}

// ConnectionsHelp is the hint line of the connection screen. "enter" connects
// and "esc" goes back — same keys, same single declaration, only the description
// follows the context. "esc" is listed only when a live session sits behind the
// screen; at startup there is nothing to go back to.
func (k KeyMap) ConnectionsHelp(canReturn, usesVault bool) []key.Binding {
	bindings := []key.Binding{
		relabel(k.AcceptSearch, "connect"),
		k.NewConn,
		relabel(k.Search, "filter"),
	}

	// Only listed on a profile that reads the vault: everywhere else the key has
	// nothing to forget.
	if usesVault {
		bindings = append(bindings, k.ForgetSecret)
	}

	if canReturn {
		bindings = append(bindings, relabel(k.Cancel, "back"))
	}

	return append(bindings, k.Quit)
}

// FormHelp is the wizard's hint line. Testing and saving are deliberately two
// keys: a profile for a host that is down is a legitimate thing to write, so
// saving does not wait on a successful test.
func (k KeyMap) FormHelp(hasSecret, revealed bool) []key.Binding {
	bindings := []key.Binding{k.NextField, k.TestConn, k.SaveConn}

	if hasSecret {
		bindings = append(bindings, relabel(k.RevealSecret, revealState(revealed)))
	}

	return append(bindings, relabel(k.Cancel, "cancel"))
}

// SecretHelp is the hint line of the password prompt. Both toggles carry their
// current state rather than only their action: whether the password is about to
// be remembered is not something to discover after pressing enter.
func (k KeyMap) SecretHelp(store, revealed bool) []key.Binding {
	return []key.Binding{
		relabel(k.AcceptSearch, "connect"),
		relabel(k.RevealSecret, revealState(revealed)),
		relabel(k.StoreSecret, "remember: "+onOff(store)),
		relabel(k.Cancel, "cancel"),
	}
}

func revealState(revealed bool) string {
	if revealed {
		return "hide password"
	}

	return "show password"
}

func onOff(on bool) string {
	if on {
		return "on"
	}

	return "off"
}

// relabel copies a binding with a different description, for the contexts where
// the same key means something else than its default label says.
func relabel(b key.Binding, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(b.Help().Key, desc))
}

// ScrollHorizontalHint merges ScrollLeft and ScrollRight into one display-only
// entry: on a single status line the two directions read better together than as
// two entries — and announcing only one of them, as the SQL tile used to, hides
// half the command. It carries no keys, the same way ShortHelp merges j/k into
// one "move" entry; the real bindings stay the two it describes, and the overlay
// still lists them separately.
func (k KeyMap) ScrollHorizontalHint() key.Binding {
	return key.NewBinding(key.WithHelp("shift+←/→ H/L", "scroll"))
}

// FullHelp is the overlay content: everything that applies right now, grouped.
func (k KeyMap) FullHelp(ctx Context) []Section {
	paneBindings := []key.Binding{
		k.Up, k.Down, k.HalfPageDown, k.HalfPageUp, k.PageDown, k.PageUp, k.Top, k.Bottom,
	}

	if ctx.HasTabs {
		paneBindings = append(paneBindings, k.NextTab, k.PrevTab)
	}

	if ctx.IsDetail {
		paneBindings = append(paneBindings, k.Inspector)
	}

	if ctx.IsText {
		paneBindings = append(paneBindings, k.Wrap, k.ScrollRight, k.ScrollLeft)
	}

	paneTitle := "Pane"
	if ctx.Pane != "" {
		paneTitle = ctx.Pane + " pane"
	}

	// n/N only mean something where "/" searches; on a list it filters instead.
	searchBindings := []key.Binding{k.searchBinding(ctx), k.AcceptSearch, k.Cancel}
	if ctx.IsText {
		searchBindings = []key.Binding{k.searchBinding(ctx), k.NextMatch, k.PrevMatch, k.AcceptSearch, k.Cancel}
	}

	return []Section{
		{Title: "Global", Bindings: []key.Binding{
			k.Help, k.Connections, k.Refresh, k.RefreshAll, k.Zoom, k.Copy, k.Cancel, k.Quit, k.ForceQuit,
		}},
		{Title: "Panes", Bindings: []key.Binding{
			k.NextPane, k.PrevPane, k.PaneSchema, k.PaneObjects, k.PaneDetail,
		}},
		{Title: paneTitle, Bindings: paneBindings},
		{Title: "Search & filter", Bindings: searchBindings},
	}
}

// namedKeys maps the non-rune keys used above back to the press that produces
// them, so a binding selected in the help can be replayed as if it had been
// typed. A key is a rune code plus modifiers since v2 — there is no KeyType
// enum, and a modified key is its base code with a Mod flag.
var namedKeys = map[string]tea.KeyPressMsg{
	"tab":         {Code: tea.KeyTab},
	"shift+tab":   {Code: tea.KeyTab, Mod: tea.ModShift},
	"enter":       {Code: tea.KeyEnter},
	"esc":         {Code: tea.KeyEscape},
	"up":          {Code: tea.KeyUp},
	"down":        {Code: tea.KeyDown},
	"left":        {Code: tea.KeyLeft},
	"right":       {Code: tea.KeyRight},
	"shift+left":  {Code: tea.KeyLeft, Mod: tea.ModShift},
	"shift+right": {Code: tea.KeyRight, Mod: tea.ModShift},
	"pgup":        {Code: tea.KeyPgUp},
	"pgdown":      {Code: tea.KeyPgDown},
	"home":        {Code: tea.KeyHome},
	"end":         {Code: tea.KeyEnd},
	"ctrl+c":      {Code: 'c', Mod: tea.ModCtrl},
	"ctrl+d":      {Code: 'd', Mod: tea.ModCtrl},
	"ctrl+u":      {Code: 'u', Mod: tea.ModCtrl},
	"ctrl+f":      {Code: 'f', Mod: tea.ModCtrl},
	"ctrl+b":      {Code: 'b', Mod: tea.ModCtrl},
	"ctrl+s":      {Code: 's', Mod: tea.ModCtrl},
	"ctrl+t":      {Code: 't', Mod: tea.ModCtrl},
}

// Synthesize rebuilds a key press from a binding's primary key. It lets the help
// overlay run the selected entry by replaying its key through the normal
// handlers, instead of maintaining a second dispatch table of commands.
func Synthesize(b key.Binding) (tea.KeyPressMsg, bool) {
	if len(b.Keys()) == 0 {
		return tea.KeyPressMsg{}, false
	}

	name := b.Keys()[0]

	if press, ok := namedKeys[name]; ok {
		return press, true
	}

	// Text is what String() reports for a printable key, and what key.Matches
	// compares against — so a synthesized "G" is indistinguishable from a typed
	// one, without having to model the shift that produced it.
	if runes := []rune(name); len(runes) == 1 {
		return tea.KeyPressMsg{Code: runes[0], Text: name}, true
	}

	return tea.KeyPressMsg{}, false
}

// RenderShort formats bindings as a single "key: description" line.
func RenderShort(bindings []key.Binding) string {
	parts := make([]string, 0, len(bindings))

	for _, b := range bindings {
		parts = append(parts, b.Help().Key+": "+b.Help().Desc)
	}

	return strings.Join(parts, "  ·  ")
}

// TableKeyMap maps the shared bindings onto a bubbles/table, so a list scrolls
// with the same keys as everything else.
func TableKeyMap(k KeyMap) table.KeyMap {
	return table.KeyMap{
		LineUp:       k.Up,
		LineDown:     k.Down,
		PageUp:       k.PageUp,
		PageDown:     k.PageDown,
		HalfPageUp:   k.HalfPageUp,
		HalfPageDown: k.HalfPageDown,
		GotoTop:      k.Top,
		GotoBottom:   k.Bottom,
	}
}

// ViewportKeyMap maps the shared bindings onto a bubbles/viewport. Left/Right
// are the horizontal scroll ones (shift+arrows), never the plain arrows, which
// change pane; g/G have no viewport binding and are handled by the caller.
func ViewportKeyMap(k KeyMap) viewport.KeyMap {
	return viewport.KeyMap{
		PageDown:     k.PageDown,
		PageUp:       k.PageUp,
		HalfPageUp:   k.HalfPageUp,
		HalfPageDown: k.HalfPageDown,
		Down:         k.Down,
		Up:           k.Up,
		Left:         k.ScrollLeft,
		Right:        k.ScrollRight,
	}
}
