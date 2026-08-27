# Takealot Plugins

This repository is a plugin marketplace containing the read-only Takealot Shopping Assistant, compatible with both Codex and Claude Code / Claude Desktop.

## Add the marketplace

### Codex

From Codex, use **Add plugin marketplace** and enter:

```text
https://github.com/tanaka-mambinge/takealot-plugin
```

The marketplace manifest at [`.agents/plugins/marketplace.json`](.agents/plugins/marketplace.json) points Codex at the installable plugin in [`plugins/takealot/`](plugins/takealot/).

### Claude Code / Claude Desktop

From Claude Code, run:

```text
/plugin marketplace add tanaka-mambinge/takealot-plugin
/plugin install takealot@takealot-plugins
```

From Claude Desktop, use **Add plugin marketplace** and enter the same `owner/repo` or full repository URL. The marketplace manifest at [`.claude-plugin/marketplace.json`](.claude-plugin/marketplace.json) points Claude at the same plugin in [`plugins/takealot/`](plugins/takealot/), which also has its own [`plugins/takealot/.claude-plugin/plugin.json`](plugins/takealot/.claude-plugin/plugin.json) manifest.

The plugin itself is documented in [`plugins/takealot/README.md`](plugins/takealot/README.md), and its agent behavior is defined by [`plugins/takealot/skills/takealot/SKILL.md`](plugins/takealot/skills/takealot/SKILL.md).

The repository also contains the GitHub Actions workflow that publishes the CLI binaries used by the skill.
