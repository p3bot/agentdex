package catalog

// Invalid on purpose: empty name and missing provider so schema load fails.
agents: "broken-agent": {
	name: ""
	bin:  "broken"
	config: {
		global: "~/.broken"
	}
}
