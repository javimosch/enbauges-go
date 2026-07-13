// Package plugins is the explicit registry of compiled-in mini-apps.
// Adding an app = one package under plugins/<id>/ + one line here.
// Disable at runtime without rebuild via PLUGINS_DISABLED=id1,id2.
package plugins

import (
	"github.com/javimosch/enbauges-go/plugin"
	"github.com/javimosch/enbauges-go/plugins/calendar"
	"github.com/javimosch/enbauges-go/plugins/carpool"
	"github.com/javimosch/enbauges-go/plugins/interactivemap"
	"github.com/javimosch/enbauges-go/plugins/ircchat"
	"github.com/javimosch/enbauges-go/plugins/lostitems"
	"github.com/javimosch/enbauges-go/plugins/openpanneau"
	"github.com/javimosch/enbauges-go/plugins/panierlibre"
)

func All() []plugin.Plugin {
	return []plugin.Plugin{
		calendar.New(),
		carpool.New(),
		interactivemap.New(),
		ircchat.New(),
		lostitems.New(),
		openpanneau.New(),
		panierlibre.New(),
	}
}
