# Roadmap de développement

Version actionnable de la roadmap. Voir [l'index](./00-overview.md), [01 — Definition Tab](./01-definition-tab.md), [02 — Query Tool](./02-query-tool.md) et [04 — Backlog](./04-backlog.md) pour le contexte.

Chaque tâche indique les **fichiers touchés** et un **critère de done** (✅ = observable).

## Deux pistes indépendantes (ordre libre / parallélisables)

Les pistes **A (Config)** et **B (Definition Tab)** touchent des fichiers disjoints — `main.go` + un nouveau `internal/config` d'un côté ; `internal/sql` + `internal/tui` de l'autre. Aucune ne bloque l'autre : faire l'une avant l'autre n'impose pas de rework. Le séquencement « fondations d'abord » de la v1 est donc abandonné au profit de ces deux chantiers menés en parallèle.

Les phases **C → E** (Query Tool puis polish) viennent ensuite et sont, elles, séquentielles.

---

## Piste A — Config & registre de connexions

### A1. Package config
- **Fichiers** : nouveau `internal/config/config.go` ; ajout d'une dépendance TOML (ex. `github.com/BurntSushi/toml`) au `go.mod`.
- Struct `Config` avec une **liste** de connexions nommées (`[]Connection{Name, DSN | host/port/user/db/sslmode}`), lue depuis `~/.config/easypg/config.toml`. Message d'erreur clair si le fichier est absent.
- ✅ L'app lit la config au démarrage ; la 1re connexion (ou une par défaut) est utilisée.

### A2. Retirer le DSN hardcodé
- **Fichiers** : `main.go` (supprimer la const `pgUrlString`, construire le DSN depuis la connexion sélectionnée ; `sql.Connect` prend déjà une string, inchangé).
- ✅ Plus aucune string DSN en dur ; le démarrage passe par la config.

### A3. Point d'extension multi-connexions
- Le registre est déjà modélisé comme une liste ; l'UI n'expose qu'une connexion pour l'instant. Documenter le futur sélecteur de connexion (à la manière de lazygit avec plusieurs repos).
- ✅ Modèle « liste de connexions » en place, prêt pour un sélecteur UI ultérieur.
- *Backlog lié* : mots de passe dans le trousseau système (`go-keyring`, macOS/Linux) — voir [04 — Backlog](./04-backlog.md), hors périmètre de cette piste.

---

## Piste B — Finir la Definition Tab

### B1. Requêtes index & contraintes
- **Fichiers** : `internal/sql/tables.go` (ou nouveaux `internal/sql/indexes.go` / `constraints.go`).
- Ajouter `QueryTableIndexes(oid)` via `pg_index` + `pg_get_indexdef`, et `QueryTableConstraints(oid)` via `pg_constraint` + `pg_get_constraintdef`. **Standardiser sur le helper générique `makeQueryAndCollectRows[T]`** (aujourd'hui seul `QueryTableAttr` l'utilise). Peupler enfin `TableAttr.Indexes` / `TableAttr.CheckConstraints`, actuellement déclarés mais laissés à `nil`.
- ✅ `QueryTableAttr` renvoie index + contraintes peuplés pour une table (vérifiable via log ou test).

### B2. Distinguer les types d'objets
- **Fichiers** : `internal/tui/components/tableTable/tableTable.go`.
- Afficher / coloriser `Table.Type` (déjà récupéré côté SQL mais non distingué visuellement) — icône ou couleur par `relkind` (table / vue / vue matérialisée / index / séquence…).
- ✅ Table vs vue vs séquence distinguables visuellement dans le panneau table.

### B3. Panneau(x) index + contraintes
- **Fichiers** : nouveaux composants (ou extension du panneau colonnes) + `definitionTab.go` / `definitionTabActions.go` pour étendre le chain `tableAttr → SetItems`.
- ✅ Sélectionner une table affiche ses index et contraintes.

### B4. Vue « SQL » / DDL (façon pgAdmin)
- **Fichiers** : `internal/sql` (nouveau `QueryViewDef` via `pg_get_viewdef` ; reconstruction du DDL à partir des colonnes + contraintes + index pour les tables) ; `definitionTab.go` (câbler enfin le tile `"view"` — aujourd'hui cible de focus orpheline, sans `SetItems` ni case `View`).
- Cas des vues : pas de drill-down colonnes éditable, juste la définition SQL.
- ✅ Le panneau view affiche le DDL de l'objet sélectionné ; les vues montrent leur définition SQL.

### B5. Refactor opportuniste (optionnel)
- Introduire une interface `Panel` / `Tile` commune (`SetItems` / `SetSize` / `View` / `Update` / `GetSelected…`) pour uniformiser `schemaTable`, `tableTable` et `columnTile`. Renommer le fichier `columTile.go` → `columnTile.go` (faute de frappe) et aligner `columnTile` sur les deux autres composants (sous-package, émission d'event, getters). Corriger le bug de receiver-valeur de `goToNextTile` / `goToPrevTile`.
- ✅ Les 3 panneaux partagent une interface ; plus de tile orpheline.

---

## Phase C — Query Tool MVP

> Dépend d'une infra d'onglets réelle (C0), aujourd'hui non fonctionnelle.

### C0. Infra multi-onglets (prérequis)
- **Fichiers** : `internal/tui/tui.go`, `internal/tui/tab.go`.
- Aujourd'hui `tabCursor` ne change jamais et `Model.Update` / `getCurrentTab` sont câblés en dur sur `definitionTab` (`editorTab` + l'interface `CustomModel` = scaffolding mort). Rendre le dispatch générique : slot d'onglet settable, keybinding de switch (ex. `1`/`2` ou `ctrl+tab`), routage de `Update` / `View` / `SetSize` vers l'onglet actif via `CustomModel`. Brancher `editorTab`.
- ✅ On bascule Definition ↔ Query au clavier ; chaque onglet reçoit ses events et sa taille.

### C1. Décision stockage historique (design, avant de coder)
- Figer le format et l'emplacement de l'historique persistant (ex. `~/.local/state/easypg/history.jsonl`, ou SQLite) **maintenant**, pour ne pas re-designer le query tool autour ensuite. Consigner la décision dans ce fichier.
- ✅ Décision écrite, structure de données de l'historique définie.

### C2. Modèle de sessions (liste)
- **Fichiers** : nouveau `internal/tui/editorTab.go`.
- State = `[]querySession` (une seule affichée au MVP) ; chaque session = { éditeur, résultats, dernière erreur, durée d'exécution }. **Conserver les rows brutes** (`[][]any`, pas seulement des strings formatées) pour préparer l'export CSV.
- ✅ Structure de sessions en place, une session active.

### C3. Pane éditeur (basique, sans vim)
- **Fichiers** : `internal/tui/editorTab.go` (+ composant dédié).
- `bubbles/textarea` pour l'édition SQL + raccourci d'exécution (ex. `ctrl+r`).
- ✅ On tape une requête et on la lance.

### C4. Exécution + pane résultats
- **Fichiers** : `internal/sql` (nouveau `RunQuery(sql) (cols, rows, err)` — requête arbitraire non typée en struct, nouveau pattern vs les `QueryXxx` actuels) ; rendu des résultats via `bubbles/table`.
- ⚠️ La connexion est un seul `*pgx.Conn` (pas de pool) : une requête longue bloque l'UI → envisager un pool ou une exécution async avec cancel.
- ✅ Un `SELECT` affiche des résultats tabulaires + le temps d'exécution ; une erreur SQL s'affiche **sans crasher** (contraste avec les `os.Exit(1)` des fetch cmds actuels).

---

## Phase D — Query Tool avancé

- **D1.** Multi-onglets : créer / fermer / naviguer entre sessions (exploite la liste de sessions de C2).
- **D2.** Bindings vim-like dans l'éditeur (modes normal / insert, `hjkl`, `dd` / `yy`).
- **D3.** Historique persistant : écriture / lecture (format figé en C1), navigation `↑` / `↓` dans l'éditeur.
- **D4.** Résultats volumineux : pagination / streaming (curseur pgx, `LIMIT`/`OFFSET` ou fetch incrémental).

---

## Phase E — Export & polish UX (façon lazygit)

- **E1.** Export CSV des résultats (rows brutes déjà disponibles depuis C2).
- **E2.** Footer d'aide contextuel (`bubbles/help` + keymaps, raccourcis selon le panneau focus) — inexistant aujourd'hui.
- **E3.** Package `theme` / `styles` centralisant les couleurs aujourd'hui dupliquées dans 3 composants (`fg 229` / `bg 57` / border `63` / header `240`).
- **E4.** Robustesse connexion : remplacer les `os.Exit(1)` des fetch cmds par une remontée d'erreur dans l'UI ; gestion de la reconnexion (d'autant plus utile avec le multi-connexions de la piste A).

---

## Dette technique transverse (à traiter opportunément)

- `os.Exit(1)` dans `definitionTabActions.go` → remonter les erreurs dans l'UI plutôt que crasher.
- `columTile.go` mal orthographié ; `columnTile` incohérent avec les deux autres composants.
- Tile `"view"` orpheline dans `definitionTabPageTileList` (résolue par B4).
- `app.log` sans rotation ni niveau de log (voir [04 — Backlog](./04-backlog.md)).
- Un seul `*pgx.Conn` vs pool (impacte le Query Tool, phase C).
