# Design : photos sur les événements (brainstorm, 2026-07-14)

Cas d'usage réel : l'affiche. Un événement de village a une affiche (fête du
four, marché, atelier) — c'est elle qu'on veut voir dans l'agenda, pas un
pavé de texte. On dimensionne pour ça.

## Décisions de cadrage proposées

1. **Une photo par événement en v1** (« l'affiche »), pas de galerie.
   Galerie = plus tard si le besoin émerge.
2. **Cible : les événements de l'agenda partagé** (`orgevents`). Le plugin
   calendrier pourra réutiliser la même brique ensuite (l'API assets est
   générique dès le départ).
3. **La modération existante est notre garde-fou.** Une photo uploadée
   anonymement n'est JAMAIS visible publiquement avant que l'événement soit
   approuvé. Pas de nouveau workflow — on s'appuie sur pending→approved.

## La brique assets (T1 ressuscitée, simplifiée)

Node est mort : plus aucune contrainte de compat superbackend/S3. On fait
simple et souverain :

- **Stockage : disque local** (`UPLOADS_DIR`, défaut `./uploads`), volume
  Docker en prod (le nom `uploads-data` est libre). Pas de S3, pas de
  dépendance externe — sobriété.
- **Clé** = id aléatoire + extension (`events/9f3ab….jpg`). Pas d'info
  devinable, pas de nom de fichier utilisateur (injection/PII).
- **Collection `assets`** (réutilisée, nouveaux docs) :
  `{key, contentType, sizeBytes, width, height, namespace: "events",
  status: "orphan"|"linked", createdAt}`.
- **Service : `GET /public/assets/{key}`** — même convention d'URL
  qu'avant (cohérence), `Cache-Control: public, max-age=31536000, immutable`
  (les clés sont uniques → cache agressif légitime, contrairement au CSS).
- **API plugin** : `ctx.Assets.Put/Open/Delete` pour que les mini-apps
  réutilisent la brique sans réinventer.

## Upload : multipart, re-encodé, borné

`POST /api/assets` (multipart/form-data, champ `photo`) :

1. **Limites dures** : 8 MB à l'entrée, image/* uniquement.
2. **Décodage + ré-encodage systématique** (stdlib `image` +
   `golang.org/x/image/draw`, pur Go) :
   - tue les payloads polyglottes (un « jpeg » qui est aussi du HTML/JS) ;
   - **supprime l'EXIF** — crucial : les photos de téléphone embarquent la
     géolocalisation du domicile des gens (RGPD/vie privée) ;
   - normalise : JPEG qualité ~82, redimensionné à 1600 px max
     (+ vignette 400 px pour les listes). ~200-400 KB par affiche.
3. **Anti-abus** (upload anonyme = surface d'abus) :
   - rate-limit en mémoire par IP (ex. 10 uploads/h) ;
   - l'asset naît `orphan` ; il passe `linked` quand un événement le
     référence ; **GC des orphelins > 48 h** (upload abandonné) ;
   - si l'événement est **rejeté** ou supprimé → photo supprimée du disque.
4. Réponse : `{key, thumbKey, width, height}` — le formulaire l'attache à
   la proposition (`photoKey` dans le POST /api/events).

## Modèle & affichage

- `orgevents` gagne `photo: {key, thumbKey}` (optionnel).
- **Formulaire « Proposer un événement »** (/agenda) : champ fichier +
  aperçu avant envoi. Optionnel, bien sûr — zéro friction.
- **Page /agenda (SSR)** : vignette dans la carte de l'événement,
  clic → image pleine taille (lightbox minimale ou simple lien).
- **Exports ouverts** : `events.json` expose `photoUrl` absolu ;
  `events.ics` gagne `ATTACH:<url>` (standard iCalendar — les apps
  calendrier qui savent l'afficher le font gratuitement).

## Le chaînon manquant : une vraie UI de modération

Aujourd'hui la modération = curl + Bearer token. Acceptable pour du texte,
**intenable pour des images** (contenu illégal, droit à l'image : il faut
des yeux humains avant publication). Cette feature impose donc :

- **`/admin/agenda`** : page minimaliste (token en query ou saisi une fois,
  gardé en localStorage) listant les événements `pending` **avec leur
  photo**, boutons Approuver / Rejeter. ~Une vue simple, même style que le
  reste.
- Rejet → l'asset est purgé du disque immédiatement.
- C'est aussi un gain net pour la modération texte existante.

## Risques & garde-fous récapitulés

| Risque | Garde-fou |
|---|---|
| Contenu illégal/inapproprié anonyme | jamais public avant approbation humaine (UI admin avec aperçu) |
| EXIF/géoloc privée | ré-encodage systématique |
| Polyglotte/XSS via fichier | ré-encodage + Content-Type forcé + clé sans nom utilisateur |
| Remplissage disque | limites taille, rate-limit IP, GC orphelins, purge au rejet |
| Droit à l'image a posteriori | mention takedown (contact@enbauges.fr) sur /privacy + suppression admin |

## Plan de mise en œuvre proposé

1. **Brique assets** : store disque + collection + `/public/assets/` +
   `ctx.Assets` + GC orphelins. Testable isolément.
2. **Upload + ré-encodage** : `POST /api/assets` (x/image, seule nouvelle
   dépendance, pure Go).
3. **Événements** : champ `photo`, formulaire /agenda avec aperçu, SSR
   vignette + pleine taille, exports (`photoUrl`, `ATTACH`).
4. **UI admin `/admin/agenda`** : liste pending avec photos,
   approuver/rejeter (purge disque au rejet).
5. Prod : volume `uploads` dans le compose + rsync… non — juste le volume ;
   le binaire fait le reste.
