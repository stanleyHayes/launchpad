"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { MarketplaceTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function MarketplacePage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [items, setItems] = useState<MarketplaceTemplate[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function load() {
    startTransition(() => {
      void getClient().listMarketplaceTemplates().then(setItems).catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          clearSession(); router.replace("/login"); return;
        }
        setError(err instanceof ApiError ? err.message : "Unable to load marketplace");
      });
    });
  }
  useEffect(() => {
    if (!getAccessToken()) { router.replace("/login"); return; }
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  function install(id: string) {
    startTransition(() => {
      void getClient().installMarketplaceTemplate(id).then((installation) => {
        setMessage(`Installed as journey ${installation.journeyTemplateId}`);
        load();
      }).catch((err) => setError(err instanceof ApiError ? err.message : "Unable to install template"));
    });
  }

  function rate(id: string, score: number) {
    startTransition(() => {
      void getClient().rateMarketplaceTemplate(id, score).then(() => {
        setMessage("Rating saved"); load();
      }).catch((err) => setError(err instanceof ApiError ? err.message : "Unable to rate template"));
    });
  }

  function submit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const element = event.currentTarget;
    const form = new FormData(element);
    const read = (key: string) => String(form.get(key) ?? "").trim();
    startTransition(() => {
      void getClient().submitMarketplaceTemplate({
        name: read("name"), description: read("description"), category: read("category"),
        steps: [{ stepType: "task", title: read("stepTitle"), instructions: read("instructions"), dueOffsetDays: 0, config: {} }],
      }).then(() => { element.reset(); setMessage("Template submitted for platform review"); })
        .catch((err) => setError(err instanceof ApiError ? err.message : "Unable to submit template"));
    });
  }

  return (
    <div className="space-y-7">
      <Reveal><PageHeader eyebrow="Journey library" title="Template marketplace" description="Install reviewed onboarding blueprints as independent draft journeys." /></Reveal>
      {error ? <p role="alert" className="text-[var(--lp-danger)]">{error}</p> : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}
      <Reveal><Surface>
        <h2 className="text-lg font-semibold">Submit a template</h2>
        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">Share a reusable journey with the marketplace review team.</p>
        <form onSubmit={submit} className="mt-4 grid gap-3 md:grid-cols-2">
          <input className="lp-input" name="name" placeholder="Template name" required />
          <input className="lp-input" name="category" placeholder="Category" required />
          <textarea className="lp-input md:col-span-2" name="description" placeholder="Description" required />
          <input className="lp-input" name="stepTitle" placeholder="First step title" required />
          <input className="lp-input" name="instructions" placeholder="Instructions" required />
          <button disabled={pending} className="lp-btn lp-btn--secondary md:w-fit">Submit for review</button>
        </form>
      </Surface></Reveal>
      {items.length === 0 ? <Surface><EmptyState title="No published templates" description="Reviewed templates will appear here." /></Surface> : (
        <div className="grid gap-5 lg:grid-cols-2">
          {items.map((item) => (
            <Reveal key={item.id}><Surface>
              <div className="flex items-start justify-between gap-3">
                <div><p className="lp-eyebrow">{item.category}</p><h2 className="mt-2 text-xl font-semibold">{item.name}</h2></div>
                {item.featured ? <span className="rounded-full bg-[var(--lp-signal)]/15 px-2.5 py-1 text-xs font-semibold">Featured</span> : null}
              </div>
              <p className="mt-3 text-sm leading-6 text-[var(--lp-ink-muted)]">{item.description}</p>
              <p className="mt-4 text-xs text-[var(--lp-ink-muted)]">v{item.version} · {item.steps.length} steps · {item.installationCount} installs · {item.ratingAverage.toFixed(1)} / 5</p>
              <div className="mt-5 flex flex-wrap items-center gap-2">
                <button disabled={pending} onClick={() => install(item.id)} className="lp-btn lp-btn--primary">Install draft</button>
                <span className="ml-auto text-xs text-[var(--lp-ink-muted)]">Rate</span>
                {[1,2,3,4,5].map((score) => <button key={score} disabled={pending} onClick={() => rate(item.id, score)} aria-label={`Rate ${score} stars`} className="grid h-8 w-8 place-items-center rounded-full border border-[var(--lp-border)] text-xs">{score}</button>)}
              </div>
            </Surface></Reveal>
          ))}
        </div>
      )}
    </div>
  );
}
