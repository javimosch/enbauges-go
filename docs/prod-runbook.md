# Prod : enbauges.fr sur enbauges-go (cutover du 2026-07-13)

## Topologie

- **vps1** (Coolify + Traefik v3). Routage : `/data/coolify/proxy/dynamic/enbauges_platform.yml`
  → `Host(enbauges.fr)` → `http://enbauges-go:3000` (réseau `coolify-shared`).
- **Conteneur** : `enbauges-go` (image `javimosch/enbauges-go:latest`),
  compose + `.env` dans `/apps/enbauges-go/`.
- **Overlay UI** : `/apps/enbauges-go/web` et `/apps/enbauges-go/plugins/<id>/web`
  montés dans le conteneur — un `./deploy.sh ui` suffit pour un changement d'UI
  (aucun restart, cache-busting automatique).
- **Node (standby)** : `enbauges_platform` reste up. Il sert encore
  `/anomalies`, `/carte-anomalies` et `/public/assets` via `PLUGIN_PROXY`
  (dernier mini-app non migré + photos S3).
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

## Rollback (< 30 s)

```bash
ssh vps1 'cp /data/coolify/proxy/dynamic-backup-enbauges_platform.yml.pre-go \
  /data/coolify/proxy/dynamic/enbauges_platform.yml'
# Traefik recharge à chaud → enbauges.fr repointe sur le Node intact.
```

## Décommission de Node (après ~1 semaine d'observation)

Bloqué par la migration d'`anomalies-map` (brique assets T1, voir
plan-node-retirement.md). Ensuite : retirer `PLUGIN_PROXY` du `.env`,
`docker compose up -d`, stopper `enbauges_platform`, archiver le dépôt Node.
