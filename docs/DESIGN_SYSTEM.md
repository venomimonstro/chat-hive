# CHAT Design System

## UX objective

CHAT must feel immediate, quiet and obvious. The interface should reveal complexity only when the user asks for it. The visual system therefore prioritizes hierarchy, touch ergonomics and performance over decorative effects.

## Permanent navigation

Mobile: Chats, Discover, Create, Me. No fifth permanent destination.

Desktop: the same information architecture expands into navigation/list + main content + optional contextual panel.

## Tokens

The UI must consume semantic tokens rather than feature-specific colors/sizes:

- colors: background, surface, raised, text, muted, line, accent, accent-soft, danger, success, warning;
- spacing: 4/8/12/16/20/24/32/40/48;
- radius: 10/14/18/24/round;
- typography: caption/body/body-strong/title/display;
- elevation: none/raised/overlay;
- motion: fast/normal; disabled for reduced-motion users.

## Interaction rules

- Primary touch target >= 44x44px.
- Visible keyboard focus for every control.
- Loading uses skeletons in content-shaped locations; avoid blocking full-screen spinners for routine navigation.
- Destructive actions require explicit semantics and confirmation when loss is not trivially reversible.
- Empty states explain the next useful action rather than only saying there is no data.
- Sheets are preferred on mobile for contextual actions; dialogs are reserved for blocking/critical decisions.

## Accessibility

- Semantic HTML first.
- Color is never the only status signal.
- Text/background contrast must remain readable in light and dark themes.
- `prefers-reduced-motion` disables nonessential animation.
- Icon-only buttons require accessible names.

## Performance rules

- Avoid heavy UI libraries until a measured need exists.
- Prefer CSS and small typed primitives.
- Long message/feed lists will use virtualization when those screens are implemented.
