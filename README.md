Underdash
=========

Underdash is a smart coding agent that intend to be used non-interactively.

Sometimes I fire up Claude Code just to fix tiny things. Sometimes I forget how a specific flag should be passed to `curl` or `tar` and I don't want to leave my terminal just to Google it, and `codex exec "sudo make me a sandwich"` or `claude -p "what's the answer to life, the universe, and everything"` feels like too much typing. That's why I made Underdash. It is intended to be aliased to `_`, i.e. the underscore. Then it becomes `_ tar the newst directory` or `_ tell me what does this repository do`. If pressing shift and minus at the same time is still too much effort, alias it to `u`.

Usage
-----

`_ help` - prints the help message.

`_ model` - prints the model currently in use.

`_ config` - list the configurations.

Build
-----

`cd src && go build -o ..`
