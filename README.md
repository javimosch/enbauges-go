# enbauges-go

Implémentation Go clean-room de [enbauges.fr](https://enbauges.fr) — le tiers-lieu
numérique du Cœur des Bauges. Un binaire statique de ~11 MB, **drop-in sur la même
base MongoDB** que l'application Node d'origine (compatibilité bidirectionnelle
vérifiée : chaque app lit et modifie les documents créés par l'autre).

Périmètre : uniquement les fonctionnalités réellement utilisées — le cœur canvas
et le calendrier partagé. Les mini-apps plugins (open-panneau, panier-libre) et le
système d'organisations superbackend ne sont pas migrés.

## Fonctionnalités

- **Canvas** (`/`) : cartes acteurs/solutions/initiatives, votes, commentaires,
  liens entre cartes, graphe, carte Leaflet — l'UI Vue 3 CDN d'origine, servie
  telle quelle.
- **API cartes** : mêmes routes et mêmes formes JSON que l'app Node
  (`/api/cards`, `/api/links`, votes, commentaires).
- **Calendrier partagé** : agenda public (`/agenda`), proposition d'événement
  sans compte (`POST /api/events` → statut `pending`), modération par token
  admin (`/api/admin/events`), flux iCal abonnable.
- **Annuaire imprimable** (`/annuaire`) avec QR code.
- **Données ouvertes** (`/donnees-ouvertes`, `/api/export/*`) : JSON, CSV,
  GeoJSON, iCal sous Licence Ouverte 2.0.
- Pages légales statiques.

## Démarrage

```bash
go build -ldflags="-s -w" -o enbauges-go .
PORT=3000 MONGODB_URI=mongodb://127.0.0.1:27017/enbauges ADMIN_TOKEN=changez-moi ./enbauges-go
```

Variables d'environnement :

| Variable | Défaut | Rôle |
|---|---|---|
| `PORT` | `3000` | Port HTTP |
| `MONGODB_URI` | `mongodb://127.0.0.1:27017/enbauges` | Connexion MongoDB |
| `MONGODB_DB` | `enbauges` | Nom de la base |
| `ADMIN_TOKEN` | *(vide)* | Token Bearer pour la modération d'événements ; vide = modération désactivée |

## Modération du calendrier

```bash
# lister les événements en attente
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:3000/api/admin/events

# approuver / rejeter
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:3000/api/admin/events/<id>/approve
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:3000/api/admin/events/<id>/reject
```

## Mini-apps (plugins)

Deux mécanismes (voir `docs/design-plugins-and-deploy.md`) :

**Plugins compilés** — un package sous `plugins/<id>/` implémentant
`plugin.Plugin` (`Meta / Mount / Install / Bootstrap`), enregistré dans
`plugins/registry.go`. `Install` tourne une fois par version (état dans la
collection `pluginstate`), la service card s'upserte sur le canvas, l'UI du
plugin (`plugins/<id>/web/`) est embarquée **et** modifiable sur disque.
Désactivation sans rebuild : `PLUGINS_DISABLED=calendar,autre`.
Premier plugin migré : `calendar` (`/calendrier`), compatible bidirectionnel
avec le plugin Node sur la collection `calendarentries`.

**Plugins proxy** — pour les mini-apps pas encore migrées (elles peuvent être
l'app Node d'origine) :

```
PLUGIN_PROXY=open-panneau=http://127.0.0.1:3015,irc-chat=http://127.0.0.1:3021
```

`/open-panneau/*` est proxifié tel quel ; upstream mort → 502 propre.

## UI modifiable sans reshipper le binaire

Tout `web/` (cœur et plugins) est embarqué dans le binaire **mais** un fichier
présent sur disque (`WEB_DIR`, défaut `./web` ; plugins : `./plugins/<id>/web`)
prend le dessus immédiatement — les templates sont re-parsés à la volée quand
une version disque existe. D'où :

```bash
./deploy.sh ui    # rsync des fichiers UI seulement — aucun restart
./deploy.sh       # build + binaire + UI + restart systemd
./deploy.sh --dry-run
```

Config dans `.deployrc` ou env : `DEPLOY_HOST`, `DEPLOY_PATH` (défaut
`/srv/enbauges`), `DEPLOY_SERVICE` (défaut `enbauges`). Sans `WEB_DIR` sur
disque, le binaire reste 100 % autosuffisant.

## Docker

```bash
docker build -t enbauges-go .
docker run -p 3000:3000 -e MONGODB_URI=mongodb://host:27017/enbauges -e ADMIN_TOKEN=... enbauges-go
```

## Mesures (même machine, mêmes données que le benchmark Node)

|  | Node (app d'origine) | enbauges-go |
|---|---|---|
| Démarrage | 11,4 s | < 0,1 s |
| RSS repos / sous charge | 135 / 440 MB | 16 / 22 MB |
| `/api/cards` (c=20) | 262 req/s | 5 265 req/s |
| Livrable | image Docker + node_modules | binaire 10,9 MB |

Voir `docs/benchmarks/go-vs-node-2026-07.md` dans le dépôt d'origine, et
`docs/VISION.md` pour l'étoile polaire (souveraineté numérique, zéro friction,
sobriété : « une autre vallée doit pouvoir le reprendre »).
