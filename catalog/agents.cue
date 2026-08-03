package catalog

agents: "agy": {
	name:        "Antigravity CLI"
	bin:         "agy"
	description: "Google's terminal-based AI coding agent, successor to Gemini CLI."
	config: {
		global: "~/.gemini/antigravity-cli"
		local:  ".agents"
	}
	skills: {
		global: {
			native: "~/.gemini/antigravity-cli/skills"
		}
		local: {
			agents: ".agents/skills"
			alternatives: [".claude/skills", ".opencode/skills"]
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	provider: ["google"]
	homepage: "https://github.com/google-antigravity/antigravity-cli"
}

agents: "aider": {
	name:        "Aider"
	bin:         "aider"
	description: "AI pair programming in the terminal."
	config: {
		global: "~/.aider"
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	agnostic: true
	homepage: "https://github.com/Aider-AI/aider"
}

agents: "claude-code": {
	name:        "Claude Code"
	bin:         "claude"
	description: "Anthropic's agentic coding tool that runs in the terminal."
	config: {
		global: "~/.claude"
		local:  ".claude"
	}
	skills: {
		global: {
			native: "~/.claude/skills"
		}
		local: {
			native: ".claude/skills"
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	provider: ["anthropic"]
	homepage: "https://github.com/anthropics/claude-code"
}

agents: "codex": {
	name:        "Codex CLI"
	bin:         "codex"
	description: "OpenAI's coding agent that runs in the terminal."
	config: {
		global: "~/.codex"
		local:  ".codex"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
			native: "~/.codex/skills"
		}
		local: {
			agents: ".agents/skills"
			native: ".codex/skills"
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	provider: ["openai"]
	homepage: "https://github.com/openai/codex"
}

agents: "copilot": {
	name:        "GitHub Copilot CLI"
	bin:         "copilot"
	description: "GitHub's agentic coding assistant for the terminal."
	config: {
		global: "~/.copilot"
		local:  ".github"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
			native: "~/.copilot/skills"
		}
		local: {
			agents: ".agents/skills"
			native: ".github/skills"
			alternatives: [".claude/skills"]
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	provider: ["github-copilot"]
	homepage: "https://github.com/github/copilot-cli"
}

agents: "goose": {
	name:        "goose"
	bin:         "goose"
	description: "Open-source AI agent with desktop app, CLI, and API for code and workflows."
	config: {
		global: "~/.config/goose"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
			native: "~/.config/goose/skills"
			alternatives: ["~/.claude/skills", "~/.config/agents/skills"]
		}
		local: {
			agents: ".agents/skills"
			native: ".goose/skills"
			alternatives: [".claude/skills"]
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	agnostic: true
	homepage: "https://github.com/aaif-goose/goose"
}

agents: "kiro": {
	name:        "Kiro CLI"
	bin:         "kiro-cli"
	description: "AWS agentic coding CLI for terminal workflows, custom agents, and deployment pipelines."
	config: {
		global: "~/.kiro"
		local:  ".kiro"
	}
	skills: {
		global: {native: "~/.kiro/skills"}
		local:  {native: ".kiro/skills"}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	agnostic: true
	homepage: "https://kiro.dev"
}

agents: "grok": {
	name:        "Grok"
	bin:         "grok"
	description: "xAI's terminal-based AI coding assistant and agentic harness."
	config: {
		global: "~/.grok"
		local:  ".grok"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
			native: "~/.grok/skills"
			alternatives: ["~/.claude/skills", "~/.cursor/skills"]
		}
		local: {
			agents: ".agents/skills"
			native: ".grok/skills"
			alternatives: [".claude/skills", ".cursor/skills"]
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	provider: ["xai"]
	homepage: "https://x.ai/cli"
}

agents: "opencode": {
	name:        "opencode"
	bin:         "opencode"
	description: "Open-source, provider-agnostic AI coding agent for the terminal."
	config: {
		global: "~/.config/opencode"
		local:  ".opencode"
	}
	skills: {
		global: {
			agents: "~/.agents/skills"
			alternatives: ["~/.claude/skills"]
		}
		local: {
			agents: ".agents/skills"
			native: ".opencode/skills"
			alternatives: [".claude/skills"]
		}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	agnostic: true
	homepage: "https://opencode.ai"
}
