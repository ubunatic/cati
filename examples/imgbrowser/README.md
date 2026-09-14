# imgbrowser

Split-pane image browser demo combining `ubunatic.com/cati` (rendering) with
`codeberg.org/ubunatic/loom`'s `Frame`/`Box` split-pane focus routing.

- **Files** pane (left): a `loom.Choice` file list, borrowed from loom's own
  `examples/filebrowser`.
- **Preview** pane (right): a custom `loom.Widget` (`imagePreview`) that
  renders the highlighted file with `cati/v1/sextant.RenderToGrid` and blits
  the resulting `core.Grid` cells straight into the shared `loom.Canvas`,
  preserving color. (`loom.View` strips inline ANSI from its lines, so it
  can't be used for a colored render — see loom's `examples/treemap`.)

Tab switches focus between panes; arrow keys move the list cursor and the
preview updates live, even before pressing Enter. Non-image files and
directories show a short message instead of a render.

```sh
go run ./examples/imgbrowser [dir]
```

Requires a real terminal (loom opens `/dev/tty`).
