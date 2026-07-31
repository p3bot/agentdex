package catalog

// Mirrors catalog/schema.cue so fixtures validate the same contract.
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

// alternatives is priority order (first is primary fallback).
#SkillsScope: {
	agents?:       string & !=""
	native?:       string & !=""
	alternatives?: [string & !="", ...(string & !="")]
}

agents: [=~"^[a-z0-9]+(-[a-z0-9]+)*$"]: #KnownAgent
