# EasyPG — Spec initiale

Index de la spec. Voir aussi :
- [01 — Definition Tab](./01-definition-tab.md)
- [02 — Query Tool](./02-query-tool.md)
- [03 — Roadmap](./03-roadmap.md)
- [04 — Backlog & dette technique](./04-backlog.md)

## Vision

TUI de gestion PostgreSQL pensé comme une alternative légère à pgAdmin pour les usages courants (pas de couverture exhaustive), avec une UX inspirée de lazygit : navigation 100% clavier, panneaux contextuels avec focus cyclique, feedback immédiat, faible friction.

## Non-objectifs (scope volontairement restreint)

- Pas de couverture complète pgAdmin (gestion fine des rôles/permissions, monitoring, backup/restore, réplication, etc.)
- Pas d'édition de données en grille façon spreadsheet — l'outil reste orienté lecture + requêtes, pas UPDATE/INSERT via l'UI
- Pas de support Windows pour l'instant (macOS/Linux uniquement)

## Décisions d'architecture

Ces points étaient ouverts dans la v1 du spec ; ils sont tranchés maintenant pour éviter de refactorer des fonctionnalités déjà prévues :

- **Connexions** : le multi-connexions est prévu dès l'architecture (registre de connexions nommées via config, à la manière dont lazygit gère plusieurs repos), même si dans l'immédiat une seule connexion est utilisée. Le DSN hardcodé de `main.go` doit être remplacé tôt par ce mécanisme plutôt que d'être patché une première fois en config simple puis re-refactoré ensuite.
- **Query tool — sessions** : le multi-onglets (plusieurs requêtes ouvertes en parallèle, comme des onglets de navigateur) est un besoin confirmé mais pas prioritaire pour le MVP. Le state du query tool doit donc être modélisé comme une liste de sessions dès le départ, même si le MVP n'en affiche qu'une seule à l'écran — le multi-onglets sera de l'exploitation de ce modèle existant, pas un refactor.
- **Historique de requêtes** : persistant sur disque (survit au redémarrage). Le format/emplacement de stockage doit être choisi dès la conception du query tool, pas ajouté après coup.
- **Export des résultats (CSV)** : pas prioritaire pour le MVP, mais le composant résultats doit conserver les rows brutes en mémoire (pas seulement des strings déjà formatées pour l'affichage) dès sa conception, pour que l'export soit un ajout simple plus tard.

## Référence UX : lazygit

- Panneaux multiples, focus cyclique au clavier (`tab`/`shift+tab`) — déjà en place dans `definitionTab`
- Aucune interaction souris
- Footer d'aide contextuelle listant les raccourcis du panneau focus — pas encore présent, à ajouter
- Densité d'info élevée, pas de fioritures visuelles
