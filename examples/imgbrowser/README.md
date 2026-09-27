# imgbrowser

Split-pane image browser demo combining `ubunatic.com/cati` (rendering) with
`codeberg.org/ubunatic/loom`'s `Frame`/`Box` split-pane focus routing.

- **Files** pane (left): a `loom.Choice` file list, borrowed from loom's own
  `examples/filebrowser`.
- **Preview** pane (right): a custom `loom.Widget` (`imagePreview`) that
  renders the highlighted file using cati's block and composite renderers
  (`six`, `half`, `quad+`, `bars`, `all`) and blits the resulting `core.Grid`
  cells straight into the shared `loom.Canvas`, preserving RGB color.

Interactive features:
- **Tab**: switch focus between file list and preview
- **m / M**: cycle render modes (`six`, `half`, `quad+`, `bars`, `all`)
- **+ / - / 0**: zoom in, zoom out, and reset zoom
- **Arrow keys / PgUp / PgDn**: navigate file list or pan when zoomed in preview
- **p / P**: inline video playback (fast vs original speed with audio in fullscreen)
- **f**: toggle fullscreen preview
- **#**: toggle detailed render & image metadata stats
- **i**: toggle images-only filtering in file list

```sh
go run ./examples/imgbrowser [dir]
```

Requires a real terminal (loom opens `/dev/tty`).
