# Plan : retrait complet de Node.js (zéro plugin proxy)

Objectif : migrer les 7 mini-apps restantes en plugins Go compilés, puis
éteindre le processus Node. Fin de partie : un seul binaire, un seul service
systemd, `PLUGIN_PROXY` vide.

## Inventaire (audité le 2026-07-13)

| Mini-app | Routes | Modèles | Particularités | Effort |
|---|---|---|---|---|
| `lost-items` | 4 | LostItem | aucune | S |
| `carpool` | 4 | CarpoolEntry | aucune | S |
| `irc-chat` | 8 | Channel, Message | polling HTTP (pas de WebSocket) | S–M |
| `interactive-map` | 7 | MapMarker | géoloc simple | M |
| `open-panneau` | 12 | Municipality, Announcement | auth mairie (sha256 email+code), superadmin Basic auth (`ADMIN_USERNAME/PASSWORD`, `OPEN_PANNEAU_ADMIN_EMAIL`) | M |
| `panier-libre` | 18 | Provider, Basket, Booking | auth par mot de passe fournisseur, suppressions en cascade | L |
| `anomalies-map` | 10 | Anomaly (+ Asset superbackend) | **upload photo base64 → objectStorage S3 + collection `assets` + `/public/assets/:key`** | L |

Bonnes nouvelles de l'audit :
- **Les 7 vues ont zéro tag EJS** — toutes sont des pages Vue 3 CDN statiques,
  portables par simple copie (même cas que `calendar`).
- **Aucun WebSocket/SSE** — même le chat est en polling ; `net/http` suffit.
- Aucune dépendance externe hors `objectStorage` (anomalies-map uniquement).

## Travaux transverses (avant ou pendant la vague 3)

### T1. Stockage d'objets pour le plugin API
`anomalies-map` uploade des photos (base64, max 5 MB) via le service
superbackend → S3, avec un doc dans la collection `assets` et une URL publique
`/public/assets/<key>`. Côté Go, conforme à la vision (sobriété, souveraineté) :

- `plugin.Context.Assets` : interface `Put(key, contentType, data) / Open(key)`.
- Implémentation par défaut : **disque local** (`UPLOADS_DIR`, défaut
  `./uploads` — le volume `uploads-data` existe déjà en prod).
- Route cœur `GET /public/assets/{key...}` : sert d'abord le disque local,
  sinon redirige vers l'URL S3 dérivée du doc `assets` (les anciennes photos
  restent accessibles sans migration).
- Écriture : doc `assets` inséré avec la même forme que superbackend
  (key, provider:"local", contentType, sizeBytes, namespace…).
- Option ultérieure : backend S3-compatible (Garage/MinIO auto-hébergé) via env.

### T2. Aides d'auth dans le package `plugin`
- `plugin.BasicAuth(user, pass)` middleware (superadmin open-panneau).
- `plugin.SHA256Hex(s)` (codes d'accès mairies, mots de passe fournisseurs —
  mêmes hashs sha256 que Node, donc les comptes existants marchent tels quels).

### T3. Protocole de vérification par plugin (celui qui a validé `calendar`)
1. Port du modèle sur la **même collection** Mongo.
2. CRUD complet via Go sur données créées par Node, puis l'inverse.
3. Vue copiée, liens morts balayés, overlay disque testé.
4. Bascule : retirer l'entrée `PLUGIN_PROXY`, ajouter au `registry.go`.
5. Le plugin Node reste en secours une semaine (rollback = re-proxifier).

## Vagues de migration

Chaque vague se termine par : registre mis à jour, entrée proxy retirée,
vérif bidirectionnelle passée, commit.

- **Vague 1 — les simples** : `lost-items`, `carpool`. ✔ (2026-07-13)
  Portés, compatibilité bidirectionnelle vérifiée avec les plugins Node sur
  `lostitems` et `carpoolentries`. Helpers `ServeWebFile`/`DBCtx` ajoutés au
  package `plugin` au passage.
- **Vague 2 — les moyens** : `irc-chat`, `interactive-map`. ✔ (2026-07-13)
  Portés avec l'index TTL partiel (90 j) des messages, la limite anti-vandalisme
  de 5 km, et le service des assets JS sous `/plugin-static/<prefix>/`
  (même convention d'URL que le loader Node). Compat bidirectionnelle vérifiée
  sur `ircchannels`, `ircmessages`, `mapmarkers`.
- **Vague 3 — les gros** : `open-panneau` (T2 requis), `panier-libre`.
  - `panier-libre` ✔ (2026-07-13) — porté avec le verrouillage optimiste `__v`
    interopérable avec mongoose (réservations concurrentes sûres entre les
    deux apps), les cascades Provider→Baskets→Bookings, la libération de stock
    à la modification/annulation, l'upsert cross-plugin de marqueur carte.
    **Découverte d'audit** : `adminPassword` est stocké en clair côté Node
    (pas de sha256 contrairement à l'hypothèse du plan) — comportement
    préservé pour la compat ; hachage à traiter comme migration coordonnée
    après le retrait de Node.
  - `open-panneau` : restant. Auth mairie sha256 réelle + superadmin Basic
    auth (T2).
- **Vague 4 — le dépendant** : `anomalies-map` (T1 requis).
  Dernier car il impose la brique assets.

## Retrait de Node (après vague 4)

1. `PLUGIN_PROXY` vide ; enbauges-go seul en prod une semaine d'observation.
2. Décommission : service/container Node coupé, dépôt `enbauges-platform`
   archivé en lecture seule (l'historique et les docs restent la référence).
3. Reste hors périmètre mini-apps, à trancher au moment du retrait :
   - `login` / `dashboard` / `browse-orgs` / `accept-invite` (système d'orgs
     superbackend) — **déjà abandonnés par design** dans enbauges-go (le
     calendrier zéro-friction les remplace). À confirmer : aucune donnée
     d'org à préserver au-delà des événements déjà compatibles.
   - `newsletter` — soit port trivial (une collection d'emails), soit abandon ;
     décision produit à prendre en vague 3.
   - OG image / SEO helpers — re-vérifier ce que `buildSeo` produit d'utile ;
     les meta sont déjà en dur dans les vues Go.

## Estimation & risques

- ~2 700 lignes de logique Node à porter + 7 vues statiques à copier.
  Au rythme de `calendar` (porté et vérifié en une session), chaque vague
  tient dans une session de travail ; 4 à 6 sessions au total, T1 inclus.
- Risques principaux : (1) la brique assets — testable isolément avant la
  vague 4 ; (2) divergences silencieuses de validation entre mongoose et Go —
  couvertes par le protocole T3 ; (3) données legacy hétérogènes dans les
  collections — les décodeurs Go doivent tolérer les champs absents (déjà le
  cas via pointeurs/omitempty).
