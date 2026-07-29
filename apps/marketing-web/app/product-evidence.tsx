import Image from "next/image";

export type EvidenceKind = "journey" | "manager" | "assistant";

const evidence = {
  journey: {
    src: "/product-evidence/journey-orchestration.png",
    alt: "LaunchPad journey workspace showing day-one milestones, role training, manager check-ins, and overall progress.",
    label: "Journey orchestration",
    detail: "A single timeline for access, learning, introductions, and manager checkpoints.",
  },
  manager: {
    src: "/product-evidence/manager-command-center.png",
    alt: "LaunchPad manager command center showing cohort progress, team members, blockers, and approval requests.",
    label: "Manager command center",
    detail: "Progress, blockers, approvals, and upcoming conversations in one operating view.",
  },
  assistant: {
    src: "/product-evidence/knowledge-assistant.png",
    alt: "LaunchPad knowledge assistant answering an access question with verified company source citations.",
    label: "Grounded knowledge assistant",
    detail: "Every answer traces back to approved sources and offers a clear human escalation path.",
  },
} satisfies Record<
  EvidenceKind,
  { src: string; alt: string; label: string; detail: string }
>;

export function ProductEvidence({
  kind,
  priority = false,
  caption = true,
  className = "",
}: {
  kind: EvidenceKind;
  priority?: boolean;
  caption?: boolean;
  className?: string;
}) {
  const item = evidence[kind];

  return (
    <figure className={`lp-evidence ${className}`}>
      <div className="lp-evidence__frame">
        <Image
          src={item.src}
          alt={item.alt}
          width={1568}
          height={1003}
          sizes="(max-width: 768px) 100vw, (max-width: 1200px) 90vw, 1120px"
          priority={priority}
          className="h-auto w-full"
        />
      </div>
      {caption ? (
        <figcaption className="mt-4 grid gap-1 sm:grid-cols-[12rem_1fr]">
          <strong className="text-sm text-[var(--lp-ink)]">{item.label}</strong>
          <span className="text-sm leading-6 text-[var(--lp-ink-muted)]">
            {item.detail}
          </span>
        </figcaption>
      ) : null}
    </figure>
  );
}

export function evidenceForSlug(slug: string): EvidenceKind {
  if (
    slug.includes("knowledge") ||
    slug.includes("support") ||
    slug.includes("learning")
  ) {
    return "assistant";
  }

  if (
    slug.includes("manager") ||
    slug.includes("analytics") ||
    slug.includes("hr-") ||
    slug.includes("enterprise") ||
    slug.includes("security") ||
    slug.includes("regulated")
  ) {
    return "manager";
  }

  return "journey";
}
