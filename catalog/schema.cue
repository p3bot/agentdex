package catalog

// #KnownAgent is one catalog entry: the static, outside facts about an agent.
// The agent id is the map key (see agents below); there is no id field here.
// When agnostic is true the entry has no home provider and forbids provider;
// when false (the default) provider is required as the models.dev join key.
#KnownAgent: {
	name:         string & !=""
	bin:          string & !=""
	description?: string
	config: {
		global: string & !=""
		local?: string & !=""
	}
	// skills roots by scope (global = user-wide, local = project). Omit skills
	// entirely when the agent has no skills dirs. When present, at least one of
	// global/local and at least one role per scope are required: the loader
	// enforces those after decode. CUE cannot express at-least-one over optional
	// fields without leaving valid entries non-concrete (breaking evaluation),
	// so cue vet alone does not reject empty skills or empty scopes — exercise
	// the library load (catalog.dir / step 4) before publishing. primary is not
	// catalogued: the library derives it as agents, else native, else the first
	// alternative.
	skills?: {
		global?: #SkillsScope
		local?:  #SkillsScope
	}
	version?: {
		args: [string, ...string] // appended to the detected binary, e.g. ["--version"]
		pattern?: string          // optional regex to extract the version
	}
	agnostic: bool | *false
	if !agnostic {
		provider: [string, ...string] // models.dev provider ids; the join key; at least one required
	}
	homepage?: string
}

// #SkillsScope is one scope's classified skill roots. At least one role must be
// set (loader-enforced; see skills comment above). alternatives is priority
// order: alternatives[0] is the primary fallback when agents and native are unset.
#SkillsScope: {
	agents?:       string & !=""
	native?:       string & !=""
	alternatives?: [string & !="", ...(string & !="")]
}

// The map key is the agent id, the single source of identity.
agents: [=~"^[a-z0-9]+(-[a-z0-9]+)*$"]: #KnownAgent
