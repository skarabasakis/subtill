---
name: open-pencil-tailwind-shadcn-design-authoring
description: >-
  Author, review, lint, and convert OpenPencil (.fig) designs that map 1:1 to
  themeable Tailwind CSS v4 utilities and shadcn components without bracket
  classes or absolute positioning. Use whenever creating, editing, inspecting,
  or converting OpenPencil frames, components, or pages for the frontend UI.
---

# OpenPencil Tailwind & shadcn Design Authoring

This skill defines the design-to-code contract for authoring and converting OpenPencil designs into themeable React + Tailwind CSS v4 + shadcn JSX.

For generic OpenPencil CLI commands, MCP tool mechanics, `eval` scripting, and base Design JSX syntax, defer to the workspace [open-pencil](../open-pencil/SKILL.md) skill and its [design-authoring reference](../open-pencil/references/design-authoring.md). Do not duplicate generic tool workflows; apply the constraints below on top of them.

## Core Invariants

1. **Themeable via CSS variables**: Every color on every node binds to a semantic variable from `frontend/src/index.css`. Zero raw hex/RGB/oklch literals on layers.
2. **Zero bracket classes**: Every spacing, sizing, typography, radius, and border value in OpenPencil must map to a native Tailwind v4 utility class (no `w-[...]`, `text-[...]`, `bg-[...]`, `gap-[...]`).
3. **Flex and Grid only**: Every container uses auto-layout (`flex="col"` or `flex="row"` preferred; `grid` for 2D layouts). Never position layout children with `x`, `y`, or `position="absolute"`.
4. **shadcn components only**: UI controls use components from the registries and style configured in `frontend/components.json` (`style: "base-nova"`, `iconLibrary: "lucide"`). Use the `shadcn` MCP server to inspect component anatomy, variants, and sizes.
5. **OpenPencil linter clean**: Designs must pass `describe`, `lint`, and `analyze spacing` without unbound colors, missing parent auto-layout, or subpixel coordinates.

---

## 1. Top-Level Frame Dimensions & Viewport Convention

All top-level frames must enable auto-layout (`flex="col"` or `flex="row"`—never `layoutMode: NONE`) and follow these dimension rules:

| Top-Level Frame Dimensions | Meaning | React / Tailwind Root Mapping |
|----------------------------|---------|-------------------------------|
| `w={1024} h={768}` (`overflow="hidden"`) | `100vw x 100vh` full-size desktop window, **no scroll** | `flex h-screen w-screen overflow-hidden bg-background text-foreground` |
| `w={1024}` with `h !== 768` (e.g. `h="hug"` or `h={1200}`) | Full-size desktop window with **vertical scroll** | `flex min-h-screen w-full overflow-y-auto bg-background text-foreground` |
| `w < 1024` | **Modal, dialog, drawer, popover, or standalone non-window component** | Width from standard Tailwind scale (`w-64`, `w-80`, `w-96`, `max-w-sm`, `max-w-md`, `max-w-lg`) or `"hug"` |

---

## 2. Node Naming Contract (Design Node -> JSX Element)

Name every node in OpenPencil so its name deterministically dictates the emitted JSX element, component, and `id`:

### Top-Level Window Frames (`w={1024}`)
- **Default page mode**: Name the frame after the page it implements in PascalCase (e.g., `Home`, `Subscriptions`, `Settings`).
- **Alternative page mode/state**: Put the mode descriptor after a colon: `<PageName>: <Mode Descriptor>` (e.g., `Home: Blank Slate`, `Subscriptions: Empty`).

### Reusable React / shadcn Component Frames
- Name the frame after the **PascalCase React component** it represents: `Button`, `Card`, `CardHeader`, `CardTitle`, `CardContent`, `Input`, `Badge`, `Dialog`, `Select`, etc.
- Convert that frame directly to the named React component in JSX.
- **Unique instance ID (`[#id]`)**: To assign a unique identifier to an instance, append `[#id]` to the component name:
  - `Button[#new-database-button]` -> `<Button id="new-database-button">`
  - `Input[#search-subscriptions]` -> `<Input id="search-subscriptions">`

### Non-Component Structural Wrapper Frames
- When a frame wraps other elements purely for layout without representing a reusable React component, name it after the **lowercase semantic HTML tag** it renders as:
  - `div`, `main`, `header`, `section`, `nav`, `aside`, `footer`, `form`, `ul`, `li`, `span`
- Optional `[#id]` suffix is supported: `div[#hero-actions]` -> `<div id="hero-actions" className="flex ...">`.

### Leaf `Text` and `Icon` Nodes
- **`Text` nodes**: Name after the semantic HTML text tag they render as: `h1`, `h2`, `h3`, `h4`, `h5`, `h6`, `p`, `span`, `label`, `small`, `code` (optionally with `[#id]`, e.g., `h1[#app-title]`).
  - Note: When a `Text` node is the direct label inside a shadcn component (like `Button` or `Badge`), it becomes the component's `children` text in JSX.
- **`Icon` nodes**: Always use Lucide icons (`Icon name="lucide:<kebab-name>"`) matching `iconLibrary: "lucide"` in `frontend/components.json`. Name the node after the PascalCase Lucide component (e.g., `BookPlus`, `BookOpen`, or `BookPlus[#id]`) so it maps directly to `<BookPlus className="size-4" />`.

---

## 3. Themeability & Semantic Color Variables (`frontend/src/index.css`)

Designs must be multi-mode themeable (`Light` and `Dark` modes in OpenPencil matching `:root` and `.dark` in `frontend/src/index.css`):

1. **Bind variables everywhere**: Always use `designVar('<token>')` for `bg`, `stroke`, and `color`. Never place `#FFFFFF`, `#000000`, or ad-hoc grays directly on layers.
2. **Allowed semantic tokens** (defined in `frontend/src/index.css`):

| Category | OpenPencil Variable (`designVar(...)`) | Tailwind Utility |
|----------|----------------------------------------|------------------|
| Page surface & text | `background`, `foreground` | `bg-background`, `text-foreground` |
| Muted surface & secondary text | `muted`, `muted-foreground` | `bg-muted`, `text-muted-foreground` |
| Card surface & text | `card`, `card-foreground` | `bg-card`, `text-card-foreground` |
| Popover / dialog surface & text | `popover`, `popover-foreground` | `bg-popover`, `text-popover-foreground` |
| Primary action & text | `primary`, `primary-foreground` | `bg-primary`, `text-primary-foreground` |
| Secondary action & text | `secondary`, `secondary-foreground` | `bg-secondary`, `text-secondary-foreground` |
| Accent surface & text | `accent`, `accent-foreground` | `bg-accent`, `text-accent-foreground` |
| Destructive action | `destructive` | `bg-destructive`, `text-destructive` |
| Borders, inputs, focus ring | `border`, `input`, `ring` | `border-border`, `border-input`, `ring-ring` |
| Sidebar tokens | `sidebar`, `sidebar-foreground`, `sidebar-primary`, `sidebar-primary-foreground`, `sidebar-accent`, `sidebar-accent-foreground`, `sidebar-border`, `sidebar-ring` | `bg-sidebar`, `text-sidebar-foreground`, etc. |
| Data visualization | `chart-1` .. `chart-5` | `bg-chart-1` .. `bg-chart-5` |

3. **Surface/foreground pairing**: Every text or icon node must use the `*-foreground` token paired with its enclosing surface (e.g., `primary-foreground` inside `primary`, `card-foreground` or `muted-foreground` inside `card`).
4. **Verify both modes**: Before finishing any design edit, verify contrast and variable bindings in both `Light` and `Dark` collection modes.

---

## 4. Layout, Spacing, Sizing & Typography Scales (No Bracket Classes)

### Layout & Indirect Sizing
- Prefer **Flexbox** (`flex="col"` or `flex="row"`) whenever possible; use **Grid** (`grid`, `columns`, `rows`, `gap`) only when 2D track alignment is required.
- Size containers indirectly using `w="fill"`, `h="fill"`, `w="hug"`, `h="hug"`, and `grow={1}` rather than fixed pixel widths/heights.
- When a fixed dimension is required (icons, control heights, sidebar/modal widths), choose exclusively from values on the Tailwind spacing/width scale:
  - Icons: `12` (`size-3`), `14` (`size-3.5`), `16` (`size-4`), `20` (`size-5`), `24` (`size-6`)
  - Control heights: `24` (`h-6`), `28` (`h-7`), `32` (`h-8`), `36` (`h-9`), `40` (`h-10`)
  - Container / modal widths: `256` (`w-64`), `320` (`w-80`), `384` (`w-96` / `max-w-sm`), `448` (`max-w-md`), `512` (`max-w-lg`), `576` (`max-w-xl`)

### Paddings & Gaps (Multiples of 4px)
- All `gap`, `p`, `px`, `py`, `pt`, `pr`, `pb`, `pl` values must be multiples of `4` (or `2` only where matching a shadcn `base-nova` primitive like `gap-1.5` = `6px` or `px-2.5` = `10px`):
  - `4` (`1`), `8` (`2`), `12` (`3`), `16` (`4`), `20` (`5`), `24` (`6`), `32` (`8`), `40` (`10`), `48` (`12`), `64` (`16`)
- Prefer multiples of `8` (`8, 16, 24, 32`) for section/layout gaps to satisfy `openpencil analyze spacing --grid 8`.

### Typography Scale
- **Font family**: Always use `Geist` / `Geist Variable` (`font-sans` in `frontend/src/index.css`). Never use `Helvetica`, `Inter`, or `Arial`.
- **Font sizes & line heights** (must map to unbracketed `text-*` utilities):

| OpenPencil `size` (px) | Tailwind Class |
|------------------------|----------------|
| `12` | `text-xs` |
| `14` | `text-sm` |
| `16` | `text-base` |
| `18` | `text-lg` |
| `20` | `text-xl` |
| `24` | `text-2xl` |
| `30` | `text-3xl` |
| `36` | `text-4xl` |

- **Font weights**: `400` (`font-normal`), `500` (`font-medium`), `600` (`font-semibold`), `700` (`font-bold`).

### Corner Radius & Borders
- **Radius**: Align with `--radius` (`0.625rem` = `10px`) in `frontend/src/index.css`:
  - `6` -> `rounded-sm`
  - `8` -> `rounded-md`
  - `10` -> `rounded-lg`
  - `14` -> `rounded-xl`
  - `9999` -> `rounded-full`
- **Strokes**: Use `strokeWidth={1}` (`border`) or `strokeWidth={2}` (`border-2`) with `stroke={designVar('border')}`.

---

## 5. shadcn Component & Registry Rules

1. Check `frontend/components.json` (`style: "base-nova"`, `baseColor: "neutral"`, `iconLibrary: "lucide"`) and existing files in `frontend/src/components/ui/`.
2. **Query `shadcn` MCP** when designing or converting a component not yet in `frontend/src/components/ui/` to verify its official subcomponent hierarchy and default classes.
3. **Match `base-nova` component geometry in OpenPencil** so JSX conversion requires zero utility overrides on shadcn components:
   - Example (`Button` default): `h={32}` (`h-8`), `gap={6}` (`gap-1.5`), `px={10}` (`px-2.5`), `rounded={8}` (`rounded-lg`), `size={14}` (`text-sm`), `weight="medium"` (`font-medium`), icon `size={16}` (`size-4`).
4. **Emit clean component props in JSX**: Output `<Button variant="outline">` instead of restyling `<Button className="bg-background border-border ...">`.

---

## 6. Linter & Verification Checklist

Run these checks via OpenPencil MCP before completing any design or conversion task:

- [ ] `describe` on the target root frame reports **zero issues** (no child using `fill` inside a non-auto-layout parent, no subpixel positions like `347.5`).
- [ ] `lint` (and `lint_fix` if needed) confirms all coordinates are whole integers and all colors are bound to variables.
- [ ] Every frame name follows the Naming Contract (`Page[: Mode]`, `Component[#id]`, or semantic HTML tag `div`/`section`/`h1`/`p`).
- [ ] Resulting React JSX contains **zero `[...]` bracket classes**, **zero inline `style` props**, and **zero `absolute` layout positioning**.

---

## Example: Compliant Design <-> Clean JSX (`Home: Blank Slate`)

### OpenPencil Design JSX (Authoring Input)

```jsx
<Frame
  name="Home: Blank Slate"
  w={1024}
  h={768}
  flex="col"
  justify="center"
  items="center"
  gap={16}
  bg={designVar("background")}
  overflow="hidden"
>
  <Frame name="div" flex="col" w="hug" h="hug" justify="center" items="center" gap={8}>
    <Text
      name="h1"
      color={designVar("foreground")}
      size={36}
      font="Geist"
      weight="bold"
      textAlign="center"
    >
      subtill
    </Text>
    <Text
      name="p"
      color={designVar("muted-foreground")}
      size={18}
      font="Geist"
      weight="normal"
      textAlign="center"
    >
      Track your online subscriptions with ease
    </Text>
  </Frame>

  <Frame name="div[#home-actions]" flex="row" w="hug" h="hug" justify="center" items="center" gap={12}>
    <Frame
      name="Button[#new-database-button]"
      flex="row"
      w="hug"
      h={32}
      gap={6}
      px={10}
      justify="center"
      items="center"
      bg={designVar("primary")}
      rounded={8}
    >
      <Icon name="lucide:book-plus" size={16} color={designVar("primary-foreground")} />
      <Text name="span" color={designVar("primary-foreground")} size={14} font="Geist" weight="medium">
        New database
      </Text>
    </Frame>

    <Frame
      name="Button[#open-database-button]"
      flex="row"
      w="hug"
      h={32}
      gap={6}
      px={10}
      justify="center"
      items="center"
      bg={designVar("background")}
      stroke={designVar("border")}
      strokeWidth={1}
      rounded={8}
    >
      <Icon name="lucide:book-open" size={16} color={designVar("foreground")} />
      <Text name="span" color={designVar("foreground")} size={14} font="Geist" weight="medium">
        Open database
      </Text>
    </Frame>
  </Frame>
</Frame>
```

### React + Tailwind + shadcn Output (Codegen Output)

```tsx
import { BookOpen, BookPlus } from "lucide-react";

import { Button } from "@/components/ui/button";

export function HomeBlankSlate() {
  return (
    <div className="flex h-screen w-screen flex-col items-center justify-center gap-4 overflow-hidden bg-background text-foreground">
      <div className="flex flex-col items-center justify-center gap-2">
        <h1 className="text-center text-4xl font-bold">subtill</h1>
        <p className="text-center text-lg text-muted-foreground">
          Track your online subscriptions with ease
        </p>
      </div>

      <div id="home-actions" className="flex flex-row items-center justify-center gap-3">
        <Button id="new-database-button">
          <BookPlus />
          New database
        </Button>
        <Button id="open-database-button" variant="outline">
          <BookOpen />
          Open database
        </Button>
      </div>
    </div>
  );
}
```
