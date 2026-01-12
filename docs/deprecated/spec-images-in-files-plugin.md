# Spec: Images in Files Plugin

## Goal
Add image preview support to the file browser plugin using `go-termimg`.

## Features
- Detect image files by extension (.png, .jpg, .jpeg, .gif, .webp, .bmp, .ico)
- Render images in the preview pane using terminal graphics protocols
- Support:
  - Kitty (native)
  - iTerm2 (native)
  - Sixel (native)
  - Halfblocks (fallback for TUI compatibility)
- Handle large images (limit > 10MB)
- Cache rendered output for performance

## Implementation Details

### 1. Image Package (`internal/image`)
- Wrapper around `github.com/blacktop/go-termimg`
- Protocol detection
- Rendering logic with caching
- TUI-safe rendering (Halfblocks protocol)

### 2. File Browser Plugin
- Add `imageRenderer` to `Plugin` struct
- Update `LoadPreview` to detect image files
- Update `renderPreviewPane` to use `renderImagePreview` when `isImage` is true
- Add `renderImagePreview` to view.go

## Dependencies
- `github.com/blacktop/go-termimg`

## UX
- When an image is selected, the preview pane shows the rendered image.
- If the terminal supports graphics (Kitty/iTerm2/Sixel), it may render high-res.
- Otherwise, it falls back to block characters (Halfblocks) which works in most terminals.
- If image is too large, show a fallback message.