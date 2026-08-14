package catalog

// Bins use the unclaimable agentdex-fixture-* prefix so host PATH tools cannot
// satisfy detection; must match internal/catalogtest.FixtureBins.
agents: "alpha-cli": {
	name:        "Alpha CLI"
	bin:         "agentdex-fixture-alpha"
	description: "Synthetic Anthropic-backed agent."
	config: {
		global: "~/.alpha"
		local:  ".alpha"
	}
	skills: {
		global: {
			native: "~/.alpha/skills"
		}
		local: {
			native: ".alpha/skills"
		}
	}
	provider: ["anthropic"]
	homepage: "https://example.com/alpha"
}

agents: "beta-tool": {
	name: "Beta Tool"
	bin:  "agentdex-fixture-beta"
	config: {
		global: "~/.config/beta"
	}
	provider: ["openai"]
}

agents: "gamma-agent": {
	name:        "Gamma Agent"
	bin:         "agentdex-fixture-gamma"
	description: "Synthetic multi-provider agent."
	config: {
		global: "~/.gamma"
		local:  ".gamma"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
			alternatives: ["~/.claude/skills"]
		}
		local: {
			agents: ".agents/skills"
			alternatives: [".claude/skills"]
		}
	}
	provider: ["google", "openai"]
	homepage: "https://example.com/gamma"
}

agents: "delta-agent": {
	name:        "Delta Agent"
	bin:         "agentdex-fixture-delta"
	description: "Synthetic provider-agnostic agent."
	config: {
		global: "~/.delta"
		local:  ".delta"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
		}
		local: {
			agents: ".agents/skills"
		}
	}
	agnostic: true
	homepage: "https://example.com/delta"
}
