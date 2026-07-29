"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { MarketplaceTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function value(form: FormData, key: string) { return String(form.get(key) ?? "").trim(); }

export default function PlatformMarketplacePage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [items, setItems] = useState<MarketplaceTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function load() {
    setLoading(true);
    startTransition(() => {
      void getClient().listPlatformMarketplaceTemplates().then(setItems).catch((err) => {
        if (err instanceof ApiError && err.status === 401) { clearSession(); router.replace("/login"); return; }
        setError(err instanceof ApiError ? err.message : "Unable to load marketplace");
      }).finally(() => setLoading(false));
    });
  }
  useEffect(() => {
    if (!getAccessToken()) { router.replace("/login"); return; }
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  function create(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    startTransition(() => {
      void getClient().createPlatformMarketplaceTemplate({
        name: value(form, "name"), description: value(form, "description"), category: value(form, "category"),
        steps: [{ stepType: "task", title: value(form, "stepTitle"), instructions: value(form, "instructions"), dueOffsetDays: Number(value(form, "dueOffsetDays") || "0"), config: {} }],
      }).then(() => { formElement.reset(); setMessage("Official draft created"); load(); })
        .catch((err) => setError(err instanceof ApiError ? err.message : "Unable to create template"));
    });
  }

  function action(item: MarketplaceTemplate, kind: "publish" | "remove" | "feature") {
    startTransition(() => {
      const request = kind === "publish" ? getClient().publishMarketplaceTemplate(item.id)
        : kind === "remove" ? getClient().removeMarketplaceTemplate(item.id)
          : getClient().featureMarketplaceTemplate(item.id, !item.featured);
      void request.then(() => { setMessage(`Template ${kind}d`); load(); })
        .catch((err) => setError(err instanceof ApiError ? err.message : "Unable to update template"));
    });
  }

  return <div className="space-y-7">
    <Reveal><PageHeader eyebrow="Catalogue" title="Template marketplace" description="Create official templates, review customer submissions, publish versions, and feature trusted blueprints." /></Reveal>
    {error ? <p role="alert" className="text-[var(--lp-danger)]">{error}</p> : null}
    {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}
    <Reveal><Surface><h2 className="text-lg font-semibold">Create official draft</h2><form onSubmit={create} className="mt-4 grid gap-3 md:grid-cols-2">
      <input className="lp-input" name="name" placeholder="Template name" required />
      <input className="lp-input" name="category" placeholder="Category" required />
      <textarea className="lp-input md:col-span-2" name="description" placeholder="Description" required />
      <input className="lp-input" name="stepTitle" placeholder="First step title" required />
      <input className="lp-input" name="dueOffsetDays" type="number" min={0} defaultValue={0} />
      <textarea className="lp-input md:col-span-2" name="instructions" placeholder="Step instructions" required />
      <button disabled={pending} className="lp-btn lp-btn--primary md:w-fit">Create draft</button>
    </form></Surface></Reveal>
    {loading ? <div className="grid gap-4" aria-label="Loading marketplace submissions">{[0,1,2].map((item) => <Surface key={item} className="flex items-center justify-between gap-4"><div className="w-2/3 space-y-3"><div className="h-5 w-1/2 animate-pulse rounded-full bg-[var(--lp-border)]" /><div className="h-3 w-full animate-pulse rounded-full bg-[var(--lp-border)]" /></div><div className="h-10 w-28 animate-pulse rounded-[var(--lp-radius)] bg-[var(--lp-border)]" /></Surface>)}</div> : items.length === 0 ? <Surface><EmptyState title="No marketplace templates" description="Create the first official draft." /></Surface> : <div className="grid gap-4">{items.map((item) => <Surface key={item.id} className="flex flex-wrap items-center justify-between gap-4">
      <div><p className="font-semibold">{item.name} <span className="text-xs text-[var(--lp-ink-muted)]">v{item.version}</span></p><p className="text-sm text-[var(--lp-ink-muted)]">{item.category} · {item.status} · {item.official ? "official" : "customer submission"} · {item.priceCents > 0 ? `${item.currency || "USD"} ${(item.priceCents / 100).toFixed(2)}` : "free"} · {item.installationCount} installs</p></div>
      <div className="flex gap-2">
        {item.status !== "published" && item.status !== "removed" ? <button disabled={pending} onClick={() => action(item, "publish")} className="lp-btn lp-btn--primary">Publish</button> : null}
        {item.status === "published" ? <button disabled={pending} onClick={() => action(item, "feature")} className="lp-btn lp-btn--secondary">{item.featured ? "Unfeature" : "Feature"}</button> : null}
        {item.status !== "removed" ? <button disabled={pending} onClick={() => action(item, "remove")} className="lp-btn lp-btn--secondary">Remove</button> : null}
      </div>
    </Surface>)}</div>}
  </div>;
}
