# Takealot Plugins

This repository is a Codex plugin marketplace containing the read-only Takealot Shopping Assistant.

## Add the marketplace

From Codex, use **Add plugin marketplace** and enter:

```text
https://github.com/tanaka-mambinge/takealot-plugin
```

The marketplace manifest at [`.agents/plugins/marketplace.json`](.agents/plugins/marketplace.json) points Codex at the installable plugin in [`plugins/takealot/`](plugins/takealot/).

The plugin itself is documented in [`plugins/takealot/README.md`](plugins/takealot/README.md), and its agent behavior is defined by [`plugins/takealot/skills/takealot/SKILL.md`](plugins/takealot/skills/takealot/SKILL.md).

The repository also contains the GitHub Actions workflow that publishes the CLI binaries used by the skill.
