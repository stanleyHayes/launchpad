import { IconWatermark } from "@launchpad/ui";
import { Icon, type IconName } from "./ui-icon";

/**
 * FeatureCard is a single capability tile: an icon chip, a title, and a short
 * description, on an elevated surface that lifts on hover. The card's own icon
 * repeats as a faint watermark bleeding off the bottom-right corner.
 */
export function FeatureCard({
  icon,
  title,
  body,
}: {
  icon: IconName;
  title: string;
  body: string;
}) {
  return (
    <div className="lp-card lp-feature-card relative overflow-hidden p-7">
      <IconWatermark icon={icon} className="-bottom-8 -right-8 size-36 rotate-[-8deg]" />
      <span className="lp-icon-chip relative" aria-hidden="true">
        <Icon name={icon} className="h-5 w-5" />
      </span>
      <h3
        className="relative mt-5 text-lg font-semibold text-[var(--lp-ink)]"
        style={{ fontFamily: "var(--lp-font-display)" }}
      >
        {title}
      </h3>
      <p className="relative mt-2 text-sm leading-6 text-[var(--lp-ink-muted)]">{body}</p>
    </div>
  );
}
