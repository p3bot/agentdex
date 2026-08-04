// #Config is the closed schema for $XDG_CONFIG_HOME/agentdex/config.cue. Closed
// so unknown fields are load-time errors. Fields with a built-in default
// (catalog.module, color) are non-optional with a default so it materialises on
// decode when omitted; remaining fields are optional (absent means "not set").
#Config: {
	cache_ttl?: string
	catalog: {
		module: string | *"github.com/p3bot/agentdex/catalog@v1"
		dir?:   string
		ttl?:   string
	}
	models: {
		url?: string
		ttl?: string
	}
	search_dirs?: [...string]
	bin_paths?: [string]: string
	color: "auto" | "always" | "never" | *"auto"
}
