#!/usr/bin/env bash
# Deploy enbauges-go.
#
#   ./deploy.sh            # full: build linux/amd64, ship binary + web/, restart service
#   ./deploy.sh ui         # UI only: rsync web/ + plugins/*/web/ — no binary, no restart
#   ./deploy.sh --dry-run  # show what would happen (works with both modes)
#
# Config via env (or a .deployrc file next to this script):
#   DEPLOY_HOST     ssh host (e.g. vps1 or user@1.2.3.4)     [required]
#   DEPLOY_PATH     remote app dir (default /srv/enbauges)
#   DEPLOY_SERVICE  systemd unit to restart (default enbauges)
set -euo pipefail
cd "$(dirname "$0")"

[ -f .deployrc ] && source .deployrc

MODE=full
DRY=""
for arg in "$@"; do
  case "$arg" in
    ui) MODE=ui ;;
    --dry-run) DRY="--dry-run" ;;
    --host) shift_host=1 ;;
    *) if [ "${shift_host:-}" = 1 ]; then DEPLOY_HOST="$arg"; shift_host=0; fi ;;
  esac
done

DEPLOY_HOST="${DEPLOY_HOST:?set DEPLOY_HOST (env or .deployrc)}"
DEPLOY_PATH="${DEPLOY_PATH:-/srv/enbauges}"
DEPLOY_SERVICE="${DEPLOY_SERVICE:-enbauges}"

RSYNC="rsync -az --delete $DRY"

sync_ui() {
  local dirs="web"
  for d in plugins/*/web; do [ -d "$d" ] && dirs="$dirs $d"; done
  [ -z "$DRY" ] && ssh "$DEPLOY_HOST" "mkdir -p $(for d in $dirs; do printf '%q ' "$DEPLOY_PATH/$d"; done)"
  for d in $dirs; do
    echo "→ syncing $d/ …"
    $RSYNC "$d/" "$DEPLOY_HOST:$DEPLOY_PATH/$d/"
  done
}

if [ "$MODE" = ui ]; then
  sync_ui
  echo "✓ UI deployed — served live via the overlay FS, no restart needed."
  exit 0
fi

echo "→ building linux/amd64 …"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o enbauges-go.new .

echo "→ uploading binary …"
if [ -n "$DRY" ]; then
  echo "(dry-run) scp enbauges-go.new $DEPLOY_HOST:$DEPLOY_PATH/enbauges-go.new"
  echo "(dry-run) ssh $DEPLOY_HOST 'mv … && systemctl restart $DEPLOY_SERVICE'"
else
  scp -q enbauges-go.new "$DEPLOY_HOST:$DEPLOY_PATH/enbauges-go.new"
  ssh "$DEPLOY_HOST" "mv '$DEPLOY_PATH/enbauges-go.new' '$DEPLOY_PATH/enbauges-go' && chmod +x '$DEPLOY_PATH/enbauges-go' && sudo systemctl restart '$DEPLOY_SERVICE'"
fi
rm -f enbauges-go.new

sync_ui
echo "✓ full deploy done."
