# Tinland

An editor written entirely in Tin, laid out like Zed: a project panel with the folder tree, tabs, the editor, a status bar
with the panel buttons, a command palette and a project search, drawn on the GPU with Metal. The pieces: the editing engine
(`packages/textedit`), the window (`packages/gpuwin`, Objective-C classes defined from Tin that AppKit calls), the GPU canvas
(`packages/metal`), the macOS binding (`packages/appkit`), and the application (`app/`, `workspace/`, `ui/`, `render/`,
`project/`, `editor/`, `language/`; ARCHITECTURE.md says what each is for). It runs on macOS: per AGENTS.md the editor is a
development tool, and Linux is the deployment platform for servers, not for this.

```
bin/tinc -o tinland products/tinland/main.tin
./tinland [folder or file...]
```

For an application with its own Dock icon and name in the menu bar, `tools/dev/tinland_app.sh` builds `bin/Tinland.app`
(`open -a bin/Tinland.app --args /path/to/folder`). The icon (a copper T with a text caret on slate, `icon/Tinland.icns`) is drawn by
`tools/dev/tinland_icon.sh` with the same GPU scene the editor uses. Every release carries `Tinland-VERSION-darwin-arm64.zip`: unzip it and
open it (it is signed ad hoc, not with a developer identity, so the first time use right click > Open).

## Keys

| | |
|---|---|
| Cmd+O, Cmd+Shift+O | open a file, open a folder |
| Cmd+Shift+E | open a recently opened file (File > Open Recent...; the list is kept in ~/.config/tinland/recent) |
| Cmd+P, Cmd+Shift+P | go to file (fuzzy), command palette |
| Cmd+Shift+Y | reveal the file in the Finder; View > Copy File Path copies its full path |
| Cmd+B, Cmd+J, Ctrl+` | show or hide the project panel, the output panel, the terminal (a shell in the project folder: colors, history, arrows, Cmd+V pastes, drag selects and Cmd+C copies) |
| Cmd+\ , Cmd+Shift+\ | split the editor (up to four panes side by side, each with its own tabs), close the active pane |
| Cmd+= , Cmd+- , Cmd+0 | zoom in, out, reset the code font |
| Cmd+Shift+F | search in the project (Return runs it; Aa, ab and .* switch case, whole word and regular expression; Tab goes to the replace box, Return there replaces every match in the project) |
| Cmd+Alt+N, Cmd+Alt+Shift+N | new file, new folder in the selected folder (the File menu also has Rename and Delete) |
| Cmd+S (Shift: Save As), Cmd+N, Cmd+W, Cmd+Q | save, new tab, close tab, quit (each asks about unsaved changes); File > Close Other Tabs and Close All Tabs do the same for every other tab, or every tab |
| Cmd+1 to 9, Ctrl+Tab | switch tabs |
| Cmd+Z, Cmd+Shift+Z, Ctrl+T | undo, redo, transpose the characters either side of the caret (the selected text with what follows it when there is a selection) |
| Cmd+X, C, V, Ctrl+L, A | cut, copy, paste, select the caret's line, select all (a cut or copy without a selection takes the line) |
| Cmd+F, Cmd+G (Shift: backwards) | find as you type; Tab switches to the replacement, Return replaces, Cmd+Return replaces all |
| Cmd+L | go to line |
| Cmd+Shift+O | go to a symbol in the file (the declarations the compiler reports: functions, types and constants, narrowed by typing) |
| Cmd+S (Shift: Save As), Cmd+N, Cmd+W, Cmd+Q | save, new tab, close tab, quit (each asks about unsaved changes) |
| Cmd+/ | comment or uncomment lines; Edit > Sort Lines and Trim Trailing Whitespace sort the touched lines and remove their trailing spaces and tabs (the command palette has both) |
| Cmd+Alt+Down, Cmd+Alt+Up, Cmd+Shift+L | add a caret on the line below or above (typing, Backspace, Delete and the arrows then act on every caret; Escape puts them away), and select every occurrence of the selected text so typing changes them all at once |
| Cmd+D, Cmd+Shift+K, Alt+Up/Down | duplicate, delete, move lines |
| Cmd+[ and Cmd+] , Tab, Shift+Tab | unindent, indent |
| Alt+Left/Right, Cmd+Left/Right/Up/Down | by word, line start/end, document start/end |
| Alt+Delete, Alt+Backspace | delete the word after or before the caret (the line break when there is no word, the selection when there is one) |
| Cmd+Alt+R (View > Toggle Ruler), Cmd+Alt+Z (View > Toggle Word Wrap) | wrap long lines at the edge of the text area instead of scrolling sideways; Up and Down then move by rows, a click lands in the row it is on; the ruler (a vertical line at `ruler_column`, 80 by default, and at the wrap column when that one comes first) shows how far a line runs |
| View > Toggle Whitespace (also in the palette) | show spaces as dots and tabs as arrows |
| Cmd+Alt+[ , Cmd+Alt+] (Edit > Fold, Unfold, Unfold All) | fold the block at the caret (the lines indented deeper than its first line), open it; a click on the arrow in the gutter does the same. Up and Down skip a fold; a fold that would hide the caret opens; switching tabs drops the folds, and an edit that changes the number of lines moves them as if it was made at the caret |
| Cmd+Shift+I | format the file with the rules of `tin fmt` |
| Cmd+Shift+B or Cmd+click, Cmd+Alt+Left | go to the declaration of the name (in another file too), and back |
| Ctrl+Q | quick info: the signature and documentation of the name |
| Ctrl+Space, or typing a `.` | completion: names after `pkg.`, methods and fields after another dot, the package's names (Up and Down choose, Tab or Return accepts, Escape closes); in Go, Rust, TypeScript, JavaScript and Python files a language server (`gopls`, `rust-analyzer`, `typescript-language-server`, `pyright-langserver` or `pylsp`, when installed: looked for in PATH, ~/go/bin, ~/.cargo/bin, /opt/homebrew/bin) completes and underlines its problems; in other files, the words of the open files |
| Cmd+R | compile and run the file with `tinc` (output in a panel; click a compiler message to jump to it) |

Every key command is also in the menu bar. Errors are underlined while you type: after a pause the compiler checks the text (`tinc -check -json` with the
unsaved buffer as an overlay), the line gets a red mark and its message shows in the status line.

Double click selects a word, triple click a line, and the status bar counts the selection's lines and characters beside the caret ("3 lines, 11 chars selected") and the whole text's ("274 lines, 1283 words, 8214 chars") beside the file's name; the wheel scrolls what is under the mouse; the divider beside the project panel drags; the title bar moves the window (a double click zooms it); a right click in the project panel opens a menu (new file, new folder, rename, delete). The folder and the open files come back at the next start when no path is given. Files and folders dropped on the window open; a file changed by another program is read again (or flagged when it has unsaved changes); binary files are not opened; a file with CRLF line endings is saved with them. Syntax colors for Tin, Go, Rust, JavaScript and TypeScript, Python, C and C++, Java, Swift, shell, JSON, YAML and TOML, and Markdown, chosen by the file name. A minimap at the right of an editor wider than 640 points shows the whole file as bars with the part in the window shaded; a click or a drag in it scrolls. Lines git sees as added, changed or removed have a mark in the gutter.

## Without a window

`TINLAND_SCRIPT` runs a sequence of steps against the editor with no window, printing what a test needs to see
(the script language is at the top of `editor/script.tin`); `snap:path.png` draws the text area on the GPU and writes the pixels (nothing is written on a machine without a GPU).
`tests/scripts/editor.script` is the test that runs in CI on macOS (`tests/run.sh`). How the packages fit together is in
[ARCHITECTURE.md](ARCHITECTURE.md).

## Settings and keys

`Tinland > Settings...` (Cmd+,) opens `~/.config/tinland/settings`, creating it with a commented template. One `name = value` per line:
`theme = dark` or `light`, `ruler = on` (a vertical line at the column a line should not run past; off by default) with `ruler_column = 80` (that column), `autosave = on` (write the files with changes when the window has been quiet for a moment; off by default), `trim_trailing_whitespace = on` (remove the trailing spaces and tabs of every line when a file is written; off by default), `insert_final_newline = on` (end a written file with a newline when it does not have one; off by default), `font_size`, `code_font` and `ui_font` (PostScript names), and key bindings by menu title: `key.toggle-terminal =
ctrl+\`` (the title in lower case with dashes; `cmd`, `shift`, `alt` and `ctrl` then the key; `none` removes the key). Restart Tinland to
apply them.
