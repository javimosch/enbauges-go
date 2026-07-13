# Design : système de plugins + déploiement rapide (brainstorm, 2026-07-13)

Deux lacunes à combler par rapport à l'app Node :

1. **Plugins** — les 8 mini-apps (`open-panneau`, `panier-libre`, `calendar`,
   `anomalies-map`, `carpool`, `interactive-map`, `irc-chat`, `lost-items`)
   vivaient dans `plugins/<id>/` avec un contrat clair. Il faut l'équivalent Go.
2. **Déploiement rapide** — le compose de prod monte `./views`, `./public`,
   `./plugins` dans une image figée : on déployait un changement d'UI par
   simple rsync + restart, sans rebuild. Le binaire Go embarque tout
   (`go:embed`) : aujourd'hui le moindre changement CSS exige de reshipper
   le binaire.

## Rappel : le contrat de plugin Node

Chaque `plugins/<id>/index.js` exporte :

```js
{
  meta:       { id, name, version, description, routePrefix },
  routes:     expressRouter,          // monté sur routePrefix
  views:      { name: cheminEjs },
  staticPath: __dirname + '/public',  // servi sous routePrefix
  hooks: {
    install(ctx),    // une fois (upsert de la service card, seed…)
    bootstrap(ctx),  // à chaque démarrage
  }
}
```

L'état (enabled, installedAt) est persisté en base par plugin. Les plugins
s'auto-annoncent sur le canvas en upsertant une carte « service ».

## Options pour le système de plugins Go

### A. Plugins compilés : packages Go + registre (voie principale ✅)

Chaque plugin est un package sous `plugins/<id>/` qui implémente une interface :

```go
// package plugin (l'API stable exposée aux mini-apps)
type Plugin interface {
    Meta() Meta                    // ID, Name, Version, Description, RoutePrefix, ServiceCard
    Mount(mux *http.ServeMux, ctx *Context) error // routes sous RoutePrefix
    Install(ctx *Context) error    // une fois par version (état en Mongo, comme Node)
    Bootstrap(ctx *Context) error  // chaque démarrage
}

type Context struct {
    DB     *mongo.Database        // même base, collections préfixées par convention
    Log    *log.Logger            // préfixé [<id>]
    Web    fs.FS                  // web/ du plugin, avec override disque (voir §déploiement)
    Render func(w, tmplName, data) // rendu html/template avec le layout commun
    UpsertServiceCard func(Card)  // auto-annonce sur le canvas
    Env    func(key, fallback) string
}
```

Enregistrement explicite dans `plugins/registry.go` :

```go
import (
    openpanneau "github.com/javimosch/enbauges-go/plugins/open-panneau"
    calendar    "github.com/javimosch/enbauges-go/plugins/calendar"
)
var All = []plugin.Plugin{ openpanneau.New(), calendar.New() }
```

Activation **sans rebuild** via env : `PLUGINS_ENABLED=open-panneau,calendar`
(ou tout-actif par défaut + `PLUGINS_DISABLED=`). L'état install/enabled reste
en Mongo (collection `pluginstate`), comme côté Node.

- ✅ Type-safe, un seul binaire, zéro dépendance runtime, testable unitairement.
- ✅ Structure identique à Node : `plugins/<id>/{plugin.go, models.go, web/}`.
- ❌ Ajouter/retirer un plugin = rebuild (mitigé par le script de déploiement §2
  et par l'activation par env).

### B. `plugin.Open()` (.so dynamiques) — écarté

Fragile (toolchain exactement identique), casse le binaire statique et le
build Alpine/musl. Non.

### C. Plugins hors-processus (reverse proxy) — voie d'appoint ✅

Un type de plugin déclaratif « proxy » : `prefix → upstream`.

```
PLUGIN_PROXY=open-panneau=http://127.0.0.1:3020,irc-chat=http://127.0.0.1:3021
```

Le cœur Go proxifie `/open-panneau/*` vers un processus séparé — qui peut être
**l'app Node actuelle** tournant avec ses 8 plugins sur la même base Mongo
(compatibilité déjà prouvée). Intérêts :

- Migration **graduelle** : le cœur passe en Go tout de suite, chaque mini-app
  migre quand on veut, sans big-bang.
- Une mini-app expérimentale peut être écrite en n'importe quoi, déployée et
  redémarrée indépendamment, crasher sans emporter le cœur.
- ❌ Plusieurs processus à superviser ; à réserver aux apps pas encore migrées
  ou volontairement isolées.

### D. Scripting embarqué (goja/Lua) — écarté

Réimplémenter Node en moins bien à l'intérieur de Go. Complexité de pont API
énorme pour un territoire qui a besoin de simplicité.

### Recommandation

**A comme modèle principal, C comme passerelle.** L'interface `plugin.Plugin`
est le contrat structurant (séparation des préoccupations, un dossier = une
app) ; le proxy déclaratif permet de garder les mini-apps Node vivantes pendant
la transition et d'accueillir des apps externes.

## Déploiement rapide + UI modifiable sans reshipper le binaire

### Le pattern : embed avec override disque

Aujourd'hui : `//go:embed web` — tout est dans le binaire. Proposition : un
`fs.FS` en cascade, disque d'abord, embed en secours :

```go
// WEB_DIR (défaut "./web") ; si le fichier existe sur disque → servi du disque,
// sinon → version embarquée. Même mécanisme pour web/ des plugins
// (plugins/<id>/web/ sur disque).
func overlayFS(diskDir string, embedded fs.FS) fs.FS
```

- Un `rsync web/ vps:/srv/enbauges/web/` suffit pour un changement CSS/HTML —
  **aucun restart** pour les fichiers statiques et le canvas (servi brut).
- Les templates (`agenda`, `annuaire`) : re-parsés à chaque requête **quand la
  version disque existe** (elles font <1 ms ; en pur embed on garde le parse
  unique au démarrage). Simple, pas de fsnotify.
- Le binaire reste autosuffisant : sans `WEB_DIR`, tout marche comme avant.
  La promesse « une autre vallée reprend le binaire seul » tient.

### Le script : `deploy.sh`

```
./deploy.sh          # build linux/amd64 + rsync binaire + web/ + restart systemd
./deploy.sh ui       # rsync web/ + plugins/*/web/ uniquement — pas de restart
./deploy.sh --host vps1
```

Mécanique : build local `CGO_ENABLED=0 GOOS=linux`, upload vers
`enbauges-go.new`, `mv` atomique, `systemctl restart enbauges` (coupure
< 100 ms grâce au démarrage instantané — c'est le luxe que Node n'avait pas).
Variante Coolify/Docker : garder l'image fixe et monter `./web` en volume,
exactement comme le compose Node monte `./views` — le pattern overlay rend les
deux modes équivalents.

### Arborescence cible

```
enbauges-go/
  main.go, cards.go, …          # cœur (à terme : package core/)
  plugin/                       # l'API plugin (interface + Context + overlay FS)
  plugins/
    registry.go                 # imports explicites
    open-panneau/
      plugin.go                 # implémente plugin.Plugin
      models.go
      web/                      # go:embed local + override disque
  web/                          # UI du cœur
  deploy.sh
```

## Ordre de test local proposé

1. Overlay FS + `deploy.sh ui` (petit, débloque le workflow immédiatement).
2. API `plugin` + registre + état Mongo, validée en migrant **une** mini-app
   simple (candidat : `panier-libre` ou `calendar`).
3. Type proxy pour les mini-apps Node restantes.
```
