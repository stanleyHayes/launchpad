# Design System Foundation

## Direction

Confident, operational, human — not generic SaaS purple. LaunchPad should feel like a calm control tower for onboarding: clear hierarchy, strong typography, soft atmospheric backgrounds, and one primary action per view.

## Brand signals

- Product name **LaunchPad** is hero-level on marketing surfaces.
- Primary CTA language: “Start free trial” / “Book a demo”.
- Supporting line focuses on time-to-productivity, not feature laundry lists.

## Tokens (CSS variables)

```css
:root {
  --lp-ink: #0f1c2e;
  --lp-ink-muted: #4a5a6f;
  --lp-paper: #f3f6fa;
  --lp-paper-elevated: #ffffff;
  --lp-accent: #0e7c66;
  --lp-accent-hover: #0a6352;
  --lp-border: #d5dee8;
  --lp-danger: #b42318;
  --lp-warning: #b54708;
  --lp-success: #027a48;
  --lp-font-display: "Fraunces", Georgia, serif;
  --lp-font-body: "Sora", system-ui, sans-serif;
  --lp-radius: 12px;
  --lp-shadow: 0 18px 50px rgba(15, 28, 46, 0.08);
}
```

## Motion

1. Hero headline fade/slide on load (respect `prefers-reduced-motion`).
2. CTA hover lift with short transition.
3. Section reveal on scroll for capability blocks.

## Component inventory (starter)

- `Button` — primary / secondary / ghost
- `Container` — max-width content shell
- `MetricCard` — admin dashboards only (interaction/summary containers)
- `SectionHeading` — one job per section

Hero surfaces must not use cards, floating badges, or overlay chips.
