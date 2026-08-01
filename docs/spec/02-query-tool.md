# Fonctionnalité 2 — Query Tool

Statut : pas commencé. Voir [l'index](./00-overview.md) pour la vision générale et les décisions d'architecture référencées ci-dessous.

Un onglet séparé (le stub `editorTab` existe déjà dans `internal/tui/tab.go` mais n'est branché nulle part) avec deux panes :

- **Pane éditeur** : édition de requêtes SQL avec bindings vim-like (a minima modes normal/insert, `hjkl`, `dd`/`yy`) — probablement `bubbles/textarea` étendu, ou intégration d'un mode vim existant pour bubbletea
- **Pane résultats** : rendu tabulaire (réutilisation probable de `bubbles/table` comme les autres panneaux) avec scroll/pagination pour les résultats larges
- **Exécution** : raccourci dédié (ex. `ctrl+enter`), affichage des erreurs SQL et du temps d'exécution
- **Sessions** : modélisées en interne comme une liste (voir [Décisions d'architecture](./00-overview.md#décisions-darchitecture)), même si le MVP n'affiche qu'un seul onglet
- **Résultats** : le composant garde les rows brutes en mémoire, pas seulement l'affichage formaté (préparation de l'export CSV)
- **Historique** : persistant sur disque, format/emplacement à définir lors de la conception (Phase 2)
