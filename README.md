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
