# Design : le graphe en vraie 3D — « la constellation du territoire »

Constat : le mode graphe actuel (cytoscape 2D, layout cose par défaut, fond
noir) est fonctionnel mais plat et laid. La vision dit « le graphe des
connexions entre cartes est le cœur du produit, pas une visualisation
annexe » — il mérite d'être le moment *waouh* de la plateforme.

## Le concept UX : une constellation vivante

Le territoire comme un ciel étoilé qu'on explore : chaque carte est un astre,
chaque lien un fil de lumière. On tourne autour, on plonge dedans, on voit
littéralement « les liens qui font la force du territoire ».

## Moteur : options

### A. `3d-force-graph` (three.js/WebGL) — recommandé ✅

- Force-directed 3D, rotation/zoom/pan orbitaux natifs, un seul script CDN
  (cohérent avec la stack Vue-CDN, pas de bundler), ~60 lignes pour un
  résultat spectaculaire.
- Nœuds = sphères émissives + sprite de label ; **particules directionnelles
  animées le long des liens** (l'« énergie » qui circule entre acteurs) ;
  post-processing bloom possible.
- Échelle : nos dizaines de nœuds sont triviales pour WebGL (la lib tient
  des milliers).

### B. Rester 2D mais soigné (sigma.js / cytoscape stylé)

Moins de waouh, moins de risques mobile. À garder comme **fallback**, pas
comme cible.

### C. Hybride 2D/3D (même famille de libs `force-graph`)

Toggle 2D/3D ; mobile et `prefers-reduced-motion` servent la 2D. C'est A +
une porte de sortie — la bonne cible finale.

## Langage visuel

| Élément | Signification |
|---|---|
| Couleur du nœud | type : terracotta (acteurs), sauge (solutions), ambre (initiatives) — mêmes couleurs que le canvas |
| Taille du nœud | nombre de connexions (comme aujourd'hui) |
| Halo / intensité | votes de la communauté |
| Style du lien | type de relation : `renforce` (flux de particules dense), `dépend de` (pointillé orienté), `utilise` (fin), `inspire` (arc lumineux) |
| Nœuds orphelins | pulsation douce + libellé « reliez cette carte » |

Fond : dégradé nuit profonde + brouillard de distance + étoiles discrètes
(pas de texture lourde). Auto-orbite lente quand personne n'interagit —
l'écran d'accueil devient un objet d'ambiance (borne du tiers-lieu ?).

## Interactions

1. **Entrée** : transition depuis le canvas — les cartes « décollent » vers
   leur position 3D (fondu simple en v1).
2. **Focus** : clic sur un nœud → la caméra vole vers lui, ses voisins
   restent éclairés, le reste s'estompe ; panneau latéral avec la carte
   (titre, description, votes, liens) + bouton « Ouvrir la carte ».
3. **Recherche** : champ de recherche → la caméra vole vers le résultat.
4. **Filtres** : puces acteurs/solutions/initiatives (mêmes toggles que le
   canvas) ; option « masquer les orphelins ».
5. **Créer du lien depuis le graphe** (l'idée produit forte) : bouton
   « Relier » sur le nœud focalisé → mode sélection → clic sur un second
   nœud → choix du type de relation → le lien naît avec ses particules.
   Le graphe cesse d'être une visualisation : il devient **l'outil qui
   fabrique la valeur** (« les liens font la valeur »).
6. **Sortie** : Échap ou ✕, comme aujourd'hui.

## Sobriété & accessibilité (garde-fous vision)

- Mobile / `prefers-reduced-motion` / WebGL absent → fallback 2D actuel
  (le bouton graphe est déjà masqué sur mobile ; on peut au contraire
  l'ouvrir en 2D).
- Auto-orbite coupée à la première interaction ; aucune animation
  indispensable à la compréhension.
- Un seul script CDN supplémentaire (~120 kB gzip incl. three.js) chargé
  **à l'ouverture du mode graphe seulement** (import dynamique), pas au
  chargement du canvas.
- Labels lisibles (sprites texte, pas de la 3D extrudée illisible).

## Plan de mise en œuvre proposé

1. **v1 (une session)** : 3d-force-graph via CDN chargé à la demande —
   sphères colorées, labels, particules sur les liens, clic → panneau
   carte, auto-orbite, filtres par type, fallback 2D conservé derrière
   un toggle.
2. **v2** : mode « Relier » (création de liens dans le graphe), recherche
   avec vol de caméra, pulsation des orphelins.
3. **v3 (plus tard)** : bloom/post-processing, entrée animée
   canvas→constellation, mode borne (plein écran auto-orbite pour un
   écran au tiers-lieu).
