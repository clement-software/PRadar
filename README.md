# Project engineering harness

Ce dépôt est un socle réutilisable pour démarrer un projet avec un workflow de
conception et d'implémentation assisté par des agents. Il ne choisit pas à
l'avance l'architecture de l'application : ce choix est pris après le PRD et le
démonstrateur, puis enregistré dans les documents d'architecture et les ADR.

## Ce que le harnais sépare

| Couche | Rôle | Emplacement |
| --- | --- | --- |
| Constitution | Invariants courts, toujours applicables | `AGENTS.md` |
| Contexte projet | Produit, domaine, architecture et décisions | `docs/` |
| Procédures | Démarrage, revue d'architecture, ticket | `.agents/workflows/` |
| Savoir générique | Skills externes, chargés à la demande | installation de l'agent |
| Enforcement | Tests, formatage, analyse statique et CI | `Makefile`, `.golangci.yml`, `.github/workflows/` |

Les corps des skills externes ne sont pas recopiés dans `AGENTS.md`. Le dépôt ne
versionne que ses propres règles, son contexte et les versions de ses outils.

## Démarrer un nouveau projet

1. Copier ce dépôt ou l'utiliser comme template Git.
2. Remplacer le nom et la description du projet dans ce fichier et dans
   `CLAUDE.md`.
3. Installer les skills décrits dans `docs/agents/skills.md`.
4. Lancer le setup en lui donnant le profil du harnais :

   ```text
   /setup-matt-pocock-skills
   Respecte docs/agents/setup-profile.md et préserve AGENTS.md.
   ```

5. Vérifier le socle avec `make doctor`.
6. Démarrer la découverte avec `/grill-with-docs`.

Le module Go `github.com/clement-software/PRadar` est initialisé ; `cmd/pradar`
est le point d'entrée du démonstrateur et `internal/` héberge ses paquets.
`make verify` et le workflow `.github/workflows/ci.yml` exécutent les mêmes
portes (`gofmt`, `go mod tidy -diff`, `go vet`, tests avec `-race`,
`golangci-lint v2.13.2` avec `modernize`).

## Démonstrateur

```sh
# analyse contrôlée de bout en bout, sans instance ni jeton
go run ./cmd/pradar run --controlled

# instance réelle : jeton lecture seule dans le trousseau macOS, puis lancement
printf '%s' "$TOKEN" | go run ./cmd/pradar token set --instance https://forge.example
go run ./cmd/pradar run --instance https://forge.example --model <modèle Claude>
```

# vérification live avant le run scoré : trousseau, Forgejo, une vraie analyse Claude, nettoyage, rendu
go run ./cmd/pradar smoke --instance https://forge.example --pull-request owner/repo#42 --model <modèle Claude>

# corpus d'évaluation : brouillon depuis les PR récentes, puis vingt PR figées
go run ./cmd/pradar corpus candidates --instance https://forge.example owner/repo-a owner/repo-b > manifest.json
#   garder 20 éléments (2 ou 3 dépôts), remplir "category" (code|ci|infra) et "reason", vérifier "size" et "authorship"
go run ./cmd/pradar corpus freeze manifest.json

Un abonnement n'est actif que si vous autorisez explicitement le moteur
configuré à lire le contenu du dépôt. Changer de moteur ou de modèle suspend la
collecte jusqu'à une nouvelle autorisation, sans perdre l'historique. Les
mesures d'usage locales s'exportent depuis `/usage.json`, et ne partent nulle
part autrement.

Le visualiseur n'écoute que sur 127.0.0.1 ; son URL est imprimée au démarrage.
Les abonnements s'ajoutent depuis la timeline en collant l'URL Forgejo du dépôt.

## Workflow de référence

```text
setup -> grill/PRD -> démonstrateur -> architecture actuelle
      -> décisions/ADR -> spec -> tickets verticaux -> TDD/review -> CI
                                      ^                         |
                                      +--- revue périodique ----+
```

Le détail des entrées, sorties et critères de passage se trouve dans
`docs/agents/workflow.md`.

## Contrats utiles

- `make doctor` valide uniquement le harnais et fonctionne avant la création du
  module Go.
- `make verify` est la porte locale complète une fois `go.mod` présent.
- `.scratch/` est le tracker local lorsque ce choix est retenu par le setup. Ses
  specs et tickets sont versionnables ; les décisions durables sont néanmoins
  promues dans `docs/`.
- Un prototype répond à une question puis vit sur une branche jetable. Le code
  validé et la décision, pas le prototype, rejoignent la branche principale.

## Base de données locale

PRadar conserve son état dans `pradar.sqlite`, sous le répertoire de données
(`--data`). Le schéma évolue par migrations ordonnées, appliquées au
démarrage :

- une base absente est créée à la version courante ;
- une base plus ancienne est migrée, après qu'une copie a été écrite à côté
  d'elle sous la forme `pradar.sqlite.v<version>.<horodatage>.backup`, dont le
  chemin est imprimé et journalisé ;
- une base écrite par une version plus récente de PRadar est refusée, jamais
  recréée ;
- une base illisible ou corrompue arrête le démarrage, les fichiers sont
  préservés et rien n'est écrit.

### Restaurer une sauvegarde

```sh
# PRadar doit être arrêté
cd "<répertoire de données>"
mv pradar.sqlite pradar.sqlite.suspect            # conserver la base en cause
rm -f pradar.sqlite-wal pradar.sqlite-shm         # journaux de la base écartée
cp pradar.sqlite.v1.20260918T090000Z.backup pradar.sqlite
```

Une copie est un fichier unique et cohérent : elle s'ouvre directement, et sera
migrée à son tour au prochain démarrage. Pour repartir de zéro, déplacez la
base hors du répertoire plutôt que de l'effacer, puis relancez : les
abonnements sont à recréer, et l'historique de la base écartée reste
consultable en la rouvrant avec une version compatible.
