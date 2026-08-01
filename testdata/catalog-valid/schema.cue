package catalog

import "struct"

// Mirror of catalog/schema.cue so fixtures share the same contract.
#KnownAgent: {
	name:         string & !=""
	bin:          string & !=""
	description?: string
	config: {
		global: string & !=""
		local?: string & !=""
	}
	skills?: {
		global?: #SkillsScope
		local?:  #SkillsScope
		struct.MinFields(1)
	}
	version?: {
		args: [string, ...string]
		pattern?: string
	}
	agnostic: bool | *false
	if !agnostic {
		provider: [string, ...string]
	}
	homepage?: string
}

// alternatives is priority order; [0] is primary fallback when agents and native are unset.
#SkillsScope: {
	agents?:       string & !=""
	native?:       string & !=""
	alternatives?: [string & !="", ...(string & !="")]
	struct.MinFields(1)
}

agents: [=~"^[a-z0-9]+(-[a-z0-9]+)*$"]: #KnownAgent
