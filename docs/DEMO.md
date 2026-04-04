# CLI Demo — VHS Recording

Generate a GIF demo of the Yertle CLI for the homepage using [VHS](https://github.com/charmbracelet/vhs) from Charm.

## How it works

1. **Write a `.tape` file** — a script describing what to type and how long to pause
2. **Run `vhs demo.tape`** — VHS launches a headless terminal, executes the script, and records a GIF
3. **Drop the GIF on the homepage** — just an `<img>` tag, no player dependencies

The `.tape` file is version-controlled, so the demo is reproducible. When the CLI changes, re-run VHS to regenerate.

## Prerequisites

```bash
brew install vhs
```

VHS also requires `ffmpeg` and `ttyd` (installed automatically by Homebrew as dependencies).

## Generating the demo

```bash
cd cli
vhs demo.tape
```

This produces `demo.gif` in the current directory.

## Embedding on the homepage

```html
<img src="/cli/demo.gif" alt="Yertle CLI demo" />
```

Or in markdown:
```markdown
![Yertle CLI demo](cli/demo.gif)
```

## Updating the demo

1. Edit `demo.tape` to change the script
2. Make sure the local backend is running (the demo hits real API endpoints)
3. Run `vhs demo.tape`
4. Commit the new `demo.gif`

## Tape file reference

Key VHS commands:
- `Type "command"` — types text into the terminal
- `Enter` — presses enter
- `Sleep 2s` — pauses for 2 seconds
- `Set FontSize 14` — terminal font size
- `Set Width 1200` / `Set Height 800` — terminal dimensions in pixels
- `Output demo.gif` — output file name

Full docs: https://github.com/charmbracelet/vhs

---

## Alternative: asciinema

If VHS doesn't work out (e.g. rendering issues, ffmpeg problems), [asciinema](https://asciinema.org/) is a fallback:

```bash
brew install asciinema
asciinema rec demo.cast    # record a live session
```

Embed with their JS player:
```html
<script src="https://asciinema.org/a/CAST_ID.js" id="asciicast-CAST_ID" async></script>
```

Trade-offs vs VHS:
- Requires embedding a third-party JS player (VHS is just a GIF)
- Not scripted — you record live, so it's harder to reproduce exactly
- Users can copy text from the player (nice but rarely used for demos)
- No ffmpeg dependency
