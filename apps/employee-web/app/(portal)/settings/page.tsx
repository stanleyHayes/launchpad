import { Icon, PageHeader, Reveal, Surface, ThemeTiles } from "@launchpad/ui";

export default function SettingsPage() {
  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          eyebrow="Account"
          title="Settings"
          description="Choose how LaunchPad looks on this device."
        />
      </Reveal>

      <Reveal delay={1}>
        <div className="max-w-3xl">
          <Surface>
            <h2 className="flex items-center gap-2 text-sm font-bold">
              <Icon name="sparkles" className="h-4 w-4 text-[var(--lp-brand)]" />
              Design system
            </h2>
            <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
              Your selection applies to every LaunchPad app on this device.
            </p>
            <div className="mt-4">
              <ThemeTiles />
            </div>
          </Surface>
        </div>
      </Reveal>
    </div>
  );
}
