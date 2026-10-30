package main

import (
	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

// buildAppService wires the enrichment registry and the AppService around the
// plugin manager and database, then points the manager's lifecycle hooks at
// the service so plugin loads keep their metadata and enrichment providers
// registered.
//
// The enrichment registry starts empty: every provider is plugin-declared and
// registers itself when its plugin first loads, so changing what a source
// returns is a plugin edit, not a host rebuild.
func buildAppService(
	db *database.DB,
	mgr *pluginmanager.Manager,
	proxy *hostnet.Proxy,
	cfgPath, cacheDir string,
) *bridge.AppService {
	enrichReg := enrich.NewRegistry()

	svc := bridge.NewAppService(db, mgr, proxy, cfgPath, cacheDir, enrichReg)
	svc.LogEnhanceStatus()
	svc.SeedVerifyCookies() // re-seed saved human-verify cookies into the proxy
	mgr.SetOnLoad(svc.SyncPluginMeta)
	mgr.SetEnrichRegistry(enrichReg)
	return svc
}
