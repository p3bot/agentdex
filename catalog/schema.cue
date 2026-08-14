package catalog

import "struct"

#KnownAgent: {
	name:         string & !=""
	bin:          string & !=""
	description?: string
	config: {
		global: string & !=""
		local?: string & !=""
	}
	// primary is not catalogued; library derives agents → native → alternatives[0].
	skills?: {
		global?: #SkillsScope
		local?:  #SkillsScope
		struct.MinFields(1)
	}
	agnostic: bool | *false
	if !agnostic {
		provider: [string, ...string] // models.dev join key
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

// Map key is the agent id (no id field on the entry).
agents: [=~"^[a-z0-9]+(-[a-z0-9]+)*$"]: #KnownAgent
