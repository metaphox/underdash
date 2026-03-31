Underdash
=========

Underdash is a smart coding agent that intend to be used non-interactively.

Sometimes I found myself get into Claude Code or Codex just for fixing tiny things. Sometimes I forgot how a specific flag should be passed to `curl` or `tar` and I don't want to leave my terminal to Google, and `codex exec "sudo make me a sandwich"` or `claude -p "what's the answer to life, the universe, and everything"` feels like too much typing. That's why I made Underdash. It should be aliased to `_`, i.e. the underscore. Then it becomes `_ tar this directory` or `_ what does this repository do`. If pressing shift and minus at the same time is too much effort, alias it to `u`.

Usage
-----

`_ help` - prints the help message.
`_ model` - prints the model currently in use.
`_ config` - list the configurations.
