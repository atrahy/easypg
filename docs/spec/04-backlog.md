# Backlog & dette technique

Voir [l'index](./00-overview.md) pour la vision générale.

## Idées en vrac (backlog non priorisé)

Pas encore creusées, juste notées pour ne pas les perdre :

- **Navigation en arbre façon pgAdmin (object explorer)** : remplacerait les 3 panneaux empilés schema/table/colonne de la Definition Tab par un seul arbre repliable (schema > tables/vues/... > colonnes/index/contraintes). Contredit l'architecture actuelle à panneaux séparés — à challenger plus tard plutôt qu'à adopter par défaut, notamment vu l'ajout prévu des panneaux index/contraintes en [Phase 1](./03-roadmap.md).
- **Mots de passe DB dans le trousseau du système** (Keychain macOS / Secret Service Linux — pas de Windows pour l'instant, voir non-objectifs) plutôt qu'en clair dans le fichier de config des connexions. S'attache naturellement au travail de la [Phase 0](./03-roadmap.md) sur le registre de connexions : le fichier de config référencerait une connexion, le mot de passe irait dans le trousseau (ex. lib `zalando/go-keyring`, dont l'usage serait restreint à macOS/Linux ici).

## Dette technique à garder en tête

- `app.log` accumule sans rotation ni niveau de log
