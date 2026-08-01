# Fonctionnalité 1 — Definition Tab (visualisation)

Statut : en cours. Voir [l'index](./00-overview.md) pour la vision générale.

État actuel (`internal/tui/definitionTab.go` + `internal/sql`) :
- Panneau schema : liste des namespaces ✅
- Panneau table : liste des tables/vues/etc. par schema ✅ (le type — table/view/index/sequence/... — est déjà récupéré côté SQL mais pas distingué visuellement)
- Panneau colonnes : nom/type/default/not null ✅

Reste à faire :
- Distinguer visuellement les types d'objets dans le panneau table (icône/couleur selon `Table.Type`)
- Panneau index (`pg_index` + `pg_get_indexdef`)
- Panneau contraintes (`pg_constraint` + `pg_get_constraintdef`) — `TableAttr.CheckConstraints` existe déjà dans le struct mais n'est pas peuplé par `QueryTableAttr`
- Onglet "définition" façon pgAdmin (SQL tab) : DDL reconstruit à partir des colonnes + contraintes + index pour une table, et de `pg_get_viewdef` pour une vue
- Cas particulier des vues : pas de panneau colonnes éditable, juste la définition SQL

---

## Design — restructuration (piste B, jet B1→B3)

Ce jet couvre l'affichage des **index** et **contraintes** (B1 + B3) et remplace la distinction visuelle des types par une **séparation en onglets** (B2 reformulé). Il s'accompagne d'une refonte du layout, plus proche de pgAdmin/lazygit.

**Hors périmètre de ce jet** (voir [roadmap](./03-roadmap.md)) :
- Vue "generated SQL"/DDL façon pgAdmin (B4) — sera une vue *togglable* (preview SQL de la ressource courante), pas un panneau permanent.
- Support des **functions** (`pg_proc`) — l'onglet Function existe mais reste vide ("à venir").
- Reconstruction du DDL des tables.

### Layout cible

Navigation à **gauche**, détail (plus grand) à **droite** :

```
┌ nav ────────────────────────┐ ┌ détail ──────────────────────────────┐
│ ┌─────────────────────────┐ │ │ [ Colonne | Index | Contraintes ]    │
│ │ schema (≈1/3 h, compact)│ │ │                                      │
│ │ scroll, N lignes (const)│ │ │   <contenu onglet détail actif>      │
│ └─────────────────────────┘ │ │                                      │
│ ┌─────────────────────────┐ │ │                                      │
│ │ objets (≈2/3 h)         │ │ │                                      │
│ │ [ Table | View | Func ] │ │ │                                      │
│ │ <liste onglet actif>    │ │ │                                      │
│ └─────────────────────────┘ │ │                                      │
└─────────────────────────────┘ └──────────────────────────────────────┘
```

- **Colonne gauche = navigation**
  - en haut (~1/3 hauteur) : panneau **schema** compact, ~4-5 lignes visibles avec scroll — hauteur configurable via une constante (`schemaVisibleRows`).
  - en bas (~2/3 hauteur) : panneau **objets** avec onglets internes **Table / View / Function** (Function en stub).
- **Colonne droite = détail** (le panneau le plus grand) : onglets internes **Colonne / Index / Contraintes** de l'objet sélectionné, **adaptatifs** selon le type.

### Interaction clavier

- `tab` / `shift+tab` : cycle le focus entre panneaux `schema → objets → détail` (mécanisme de focus existant, redéfini pour 3 panneaux).
- `[` / `]` : change l'onglet interne du panneau focus (façon lazygit) — s'applique au panneau **objets** (Table/View/Function) et au panneau **détail** (Colonne/Index/Contraintes).
- `h` / `l` (alias de nav entre panneaux) : **non implémentés pour l'instant**, notés pour plus tard.

### Détail adaptatif selon le type d'objet

- **Table** → onglets Colonne / Index / Contraintes.
- **View** → onglet Colonne seul (index/contraintes masqués).
- **Function** → définition (n/a pour ce jet, onglet stub).

### Découpage technique

**B1 — SQL : index & contraintes**
- Nouveau `internal/sql/indexes.go` : `IndexAttr{ Name, Definition, IsPrimary, IsUnique }` + `QueryTableIndexes(oid)` via `pg_index` + `pg_get_indexdef(indexrelid)`, filtré `indrelid = $1`.
- Nouveau `internal/sql/constraints.go` : `ConstraintAttr{ Name, Type, Definition }` (Type = `contype` mappé check/fk/pk/unique…) + `QueryTableConstraints(oid)` via `pg_constraint` + `pg_get_constraintdef(oid)`, filtré `conrelid = $1`.
- Les deux réutilisent le helper générique `makeQueryAndCollectRows[T]` (`internal/sql/connection.go`).
- `internal/sql/tables.go` : redéfinir `TableAttr` en `{ Columns []ColumnAttr; Indexes []IndexAttr; Constraints []ConstraintAttr }` (remplace les `[]string` aujourd'hui déclarés mais jamais peuplés) ; étendre `QueryTableAttr(oid)` pour appeler les 3 requêtes et peupler la struct.

**Composants TUI**
- Nouveau helper d'onglets réutilisable `internal/tui/components/tabs/` : liste de labels + index actif, `Next()`/`Prev()` (mappés sur `]`/`[`), rendu du header `[ Colonne | Index | Contraintes ]` avec surbrillance de l'actif, support des onglets masqués (pour l'adaptatif). Réutilisé par le panneau objets **et** le panneau détail.
- `detailPane` (droite) : compose `columnTile` + nouveaux `indexTile`/`constraintTile` (miroirs de `columnTile`, wrap `bubbles/table`) + un `tabs` interne ; `SetItems(attr, objType)` peuple les tiles et fixe les onglets visibles selon le type.
- `objectsPane` (gauche-bas) : onglets Table/View/Function ; `SetItems([]sql.Table)` **partitionne** par `Table.Type` (`table`/`partitioned table` → Table ; `view`/`materialized view` → View ; autres kinds ignorés pour l'instant) ; émet un event "objet sélectionné" (nom + OID + type) sur mouvement de curseur ou changement d'onglet.

**Chaîne de messages (rewire)**
- schema cursor → `fetchTables(schema)` → `tablesList` → `objectsPane.SetItems(...)` → auto-sélection du 1er objet → event objet-sélectionné.
- objet-sélectionné (curseur objets **ou** switch d'onglet objets) → `fetchTableAttr(oid)` → `tableAttr` → `detailPane.SetItems(attr, objType)`.
- `[`/`]` routé vers le panneau focus : sur objets → nouveau fetch ; sur détail → simple changement de vue.

**Cleanups au passage**
- Renommer `internal/tui/columTile.go` → `columnTile.go`.
- Corriger le receiver-valeur de `goToNextTile`/`goToPrevTile` (absorbé par la nouvelle logique de focus à 3 panneaux).
