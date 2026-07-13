# Prod : enbauges.fr sur enbauges-go (cutover du 2026-07-13)

## Topologie

- **vps1** (Coolify + Traefik v3). Routage : `/data/coolify/proxy/dynamic/enbauges_platform.yml`
  → `Host(enbauges.fr)` → `http://enbauges-go:3000` (réseau `coolify-shared`).
- **Conteneur** : `enbauges-go` (image `javimosch/enbauges-go:latest`),
  compose + `.env` dans `/apps/enbauges-go/`.
- **Overlay UI** : `/apps/enbauges-go/web` et `/apps/enbauges-go/plugins/<id>/web`
  montés dans le conteneur — un `./deploy.sh ui` suffit pour un changement d'UI
  (aucun restart, cache-busting automatique).
- **Node : décommissionné le 2026-07-13.** Conteneur arrêté et retiré ;
  `anomalies-map` abandonné (décision produit), `PLUGIN_PROXY` vide.
  Les fichiers restent dans `/apps/enbauges_platform` (archive froide).
- **Mongo** : inchangé (mongo prod :27019) — les deux apps lisent/écrivent la même base.
- **ADMIN_TOKEN** (modération agenda) : dans `/apps/enbauges-go/.env` sur vps1.

## Déployer

```bash
# Binaire/plugins (nouvelle image) :
docker build -t javimosch/enbauges-go:latest . && docker push javimosch/enbauges-go:latest
ssh vps1 'cd /apps/enbauges-go && docker compose pull -q && docker compose up -d'

# UI seulement (instantané) :
./deploy.sh ui
```

## Rollback d'urgence (Node est arrêté, plus chaud)

```bash
# 1. Relancer Node (fichiers toujours sur place) :
ssh vps1 'cd /apps/enbauges_platform && docker compose -f compose.vps1.yml up -d'
# 2. Repointer Traefik :
ssh vps1 'cp /data/coolify/proxy/dynamic-backup-enbauges_platform.yml.pre-go \
  /data/coolify/proxy/dynamic/enbauges_platform.yml'
```
