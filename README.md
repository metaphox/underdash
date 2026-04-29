Underdash
=========

Underdash is a smart coding agent that intend to be used non-interactively.

Sometimes I fire up Claude Code just to fix tiny things. Sometimes I forget how a specific flag should be passed to `curl` or `tar` and I don't want to leave my terminal just to Google it, and `codex exec "sudo make me a sandwich"` or `claude -p "what's the answer to life, the universe, and everything"` feels like too much typing. That's why I made Underdash. It is intended to be aliased to `_`, i.e. the underscore. Then it becomes `_ tar the newst directory` or `_ tell me what does this repository do`. If pressing shift and minus at the same time is still too much effort, alias it to `u`.

Usage
-----

`_ help` - prints the help message.

`_ model` - prints the model currently in use.

`_ config` - list the configurations.

Examples
--------

```bash
_ :echo "text" prints "text" followed by a newline -- how to make echo print no new line?
To make echo print without a newline, use the -n flag: `echo -n "your text"`. This suppresses the trailing newline that echo normally adds. For example, `echo -n "Hello"` will print "Hello" without moving to the next line, so subsequent output will continue on the same line.
```

Note: on Zsh, this may fail as unmatched wildcards will cause an error. See **caveats** below.

Caveats
-------

In bash, if a wildcard pattern such as `?` or `*` doesn't match any file, it will be left unchanged and passed to Underdash. In zsh, however, you'll get an error:

```bash
_ what is the answer to life, the universe, and everything?
zsh: no matches found: everything?
```

usually the safest setting. If you want to pass a wildcard parameter to a command, use quotes. You can switch to the bash behavior with setopt no_nomatch.

Build
-----

`cd src && go build -o ..`
