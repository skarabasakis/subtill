---
name: open-pencil
description: Work with Figma .fig design files and the running OpenPencil editor — inspect structure, query nodes, analyze design tokens, export PNG/SVG/PDF/JSX, and modify designs programmatically. Use when asked to open, inspect, export, analyze, or edit .fig files, or to control the running OpenPencil app.
---

# OpenPencil

### MCP workflow

1. **Find or open a document** — `list_documents` for open tabs and their IDs; `open_file { path }` within the effective filesystem root (`list_recent_files` lists recently opened files there), or `new_document {}`. Pass `document_id` and `page_id` explicitly instead of relying on the active tab; `activate_document { document_id }` brings a tab to the front when the user should see it.
2. **Query** — `get_page_tree`, `find_nodes`, `query_nodes`, `get_node`, `list_pages`, `get_current_page`.
3. **Inspect** — `get_jsx`, `diff_jsx`, `diff_create`, `diff_visual`, `describe`, `export_image`, `export_svg`, `export_pdf`.
4. **Modify** — `render`, `batch_update`, `update_node`, `set_fill`, `set_layout`, `create_shape`, `import_svg`, etc.
5. **Save/export** — `save_file`, `export_image`, `export_svg`, `export_pdf`, or CLI `export`.
6. **Close** — `close_file { document_id, unsaved }` closes a document tab after the workflow. With unsaved changes it fails unless `unsaved` is `"save"` or `"discard"`; automation never prompts in the app.

Use `undo` / `redo { document_id }` to step back your own changes. They refuse when the newest step was made by the user in the editor; never work around that. `get_settings` and `update_settings { settings }` read and change editor preferences such as theme, language, and snapping; they never expose credentials, models, or tool access.

## Key tools for agents

- **`query_nodes`** — XPath selectors to find specific nodes without fetching the full tree.
- **`get_jsx`** — inspect any node as JSX in the same format accepted by `render`.
- **`diff_jsx` / `diff_create`** — compare two nodes as a JSX line diff or as an appliable patch of JSX attributes; `diff_show` previews setting attributes and `diff_apply` applies a patch only if the nodes still match it.
- **`diff_visual`** — pixel diff between two rendered nodes; use it to confirm an edit changed only the intended region.
- **`describe`** — semantic analysis of role, visual style, layout, and design issues.
- **`batch_update`** — apply multiple node updates efficiently.
- **`export_image` / `export_svg` / `export_pdf`** — visual verification and deliverables.
- **`viewport_zoom_to_fit` / `viewport_set` / `viewport_get`** — move the user's view only when they ask to be shown something.
- **`get_codegen_prompt`** — retrieve OpenPencil's current JSX/codegen guidance.
- **`undo` / `redo`** — revert or reapply your newest change; they refuse to touch the user's edits.
- **`list_documents` / `activate_document`** — discover open tabs and show the one you worked on.

## JSX Rendering

Read [Design authoring](references/design-authoring.md) before creating or modifying JSX designs. This bundled reference is generated from Core's authoring guidance, tested examples, and renderer metadata—the same reference used by chat and codegen prompts.

Use the `render` tool for JSX strings. Use only the APIs exposed by the installed `eval` environment; native library exports are not automatically scripting globals. The connected server's `get_codegen_prompt` provides its version's codegen and authoring guidance.

## Tips

- Omit the file path to work with the document open in the running OpenPencil editor.
- Start with `info` or `get_page_tree` to understand the document.
- Use `tree --depth 2` or `query_nodes` to avoid overwhelming output on large files.
- Export specific nodes with `--node` for faster visual checks.
- Use `export_image` after changes to verify visual quality.
- The running editor shows each MCP session as an agent at the layers its tools touch, and follows it while it works when the user has Follow agents on. Leave the user's selection and view alone unless they ask to be shown something.
- Use `analyze colors --similar` to find near-duplicate colors.
- Use `openpencil tool call` for MCP tools without a dedicated CLI command, and `eval` for Figma Plugin API operations not covered by any tool.
- Use `--json` when piping CLI output to scripts.
- In app mode, `eval` and MCP modifications are reflected live in the editor.
