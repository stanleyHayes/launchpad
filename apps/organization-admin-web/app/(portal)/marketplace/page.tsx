"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { MarketplaceTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, IconWatermark, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function money(cents: number, currency: string): string {
  if (cents === 0) return "Free";
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: currency || "USD",
    maximumFractionDigits: 2,
  }).format(cents / 100);
}

function MarketplaceSkeleton() {
  return (
    <div className="grid gap-5 lg:grid-cols-2" aria-label="Loading marketplace">
      {[0, 1, 2, 3].map((item) => (
        <Surface key={item} className="space-y-5 overflow-hidden">
          <div className="h-3 w-24 animate-pulse rounded-full bg-[var(--lp-border)]" />
          <div className="h-7 w-2/3 animate-pulse rounded-full bg-[var(--lp-border)]" />
          <div className="space-y-2">
            <div className="h-3 w-full animate-pulse rounded-full bg-[var(--lp-border)]" />
            <div className="h-3 w-4/5 animate-pulse rounded-full bg-[var(--lp-border)]" />
          </div>
          <div className="flex justify-between">
            <div className="h-9 w-32 animate-pulse rounded-[var(--lp-radius)] bg-[var(--lp-border)]" />
            <div className="h-9 w-20 animate-pulse rounded-full bg-[var(--lp-border)]" />
          </div>
        </Surface>
      ))}
    </div>
  );
}

export default function MarketplacePage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [loading, setLoading] = useState(true);
  const [items, setItems] = useState<MarketplaceTemplate[]>([]);
  const [mine, setMine] = useState<MarketplaceTemplate[]>([]);
  const [paid, setPaid] = useState(false);
  const [page, setPage] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function load() {
    setLoading(true);
    void Promise.all([
      getClient().listMarketplaceTemplates(),
      getClient().listMyMarketplaceTemplates(),
    ]).then(([catalogue, creatorItems]) => {
      setItems(catalogue);
      setPage(0);
      setMine(creatorItems);
    }).catch((err: unknown) => {
      if (err instanceof ApiError && err.status === 401) {
        clearSession(); router.replace("/login"); return;
      }
      setError(err instanceof ApiError ? err.message : "Unable to load marketplace");
    }).finally(() => setLoading(false));
  }

  useEffect(() => {
    if (!getAccessToken()) { router.replace("/login"); return; }
    const reference = new URL(window.location.href).searchParams.get("reference");
    if (reference) {
      setLoading(true);
      void getClient().completeMarketplacePurchase(reference).then((installation) => {
        setMessage(`Purchase complete. Installed as journey ${installation.journeyTemplateId}`);
        window.history.replaceState({}, "", "/marketplace");
        load();
      }).catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to verify marketplace purchase");
        setLoading(false);
      });
      return;
    }
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  function install(item: MarketplaceTemplate) {
    setError(null);
    startTransition(() => {
      const action = item.priceCents > 0
        ? getClient().purchaseMarketplaceTemplate(item.id).then((checkout) => {
            window.location.assign(checkout.authorizationUrl);
          })
        : getClient().installMarketplaceTemplate(item.id).then((installation) => {
            setMessage(`Installed as journey ${installation.journeyTemplateId}`);
            load();
          });
      void action.catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to install template");
      });
    });
  }

  function rate(id: string, score: number) {
    startTransition(() => {
      void getClient().rateMarketplaceTemplate(id, score).then(() => {
        setMessage("Rating saved"); load();
      }).catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Unable to rate template"));
    });
  }

  function submit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const element = event.currentTarget;
    const form = new FormData(element);
    const read = (key: string) => String(form.get(key) ?? "").trim();
    const priceCents = paid ? Math.round(Number(read("price")) * 100) : 0;
    setError(null);
    startTransition(() => {
      void getClient().submitMarketplaceTemplate({
        name: read("name"), description: read("description"), category: read("category"),
        priceCents, currency: read("currency") || "USD",
        steps: [{
          stepType: "task", title: read("stepTitle"), instructions: read("instructions"),
          dueOffsetDays: Number(read("dueOffsetDays") || "0"), config: {},
        }],
      }).then(() => {
        element.reset();
        setPaid(false);
        setMessage("Template submitted for marketplace review");
        load();
      }).catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Unable to submit template"));
    });
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Creator marketplace"
          title="Build once. Share it—or sell it."
          description="Turn your best onboarding journeys into reviewed templates other teams can install."
        />
      </Reveal>

      {error ? <p role="alert" className="text-[var(--lp-danger)]">{error}</p> : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

      <Reveal>
        <Surface className="overflow-hidden p-0">
          <div className="grid lg:grid-cols-[.72fr_1.28fr]">
            <div className="bg-[var(--lp-ink)] p-7 text-white">
              <p className="text-xs font-bold uppercase tracking-[.2em] text-white/60">Creator studio</p>
              <h2 className="mt-4 text-3xl font-semibold">Publish your playbook</h2>
              <p className="mt-3 leading-7 text-white/70">
                Choose free distribution to grow your reach, or set a one-time price. LaunchPad retains a 15% marketplace fee on paid sales.
              </p>
              <div className="mt-8 grid grid-cols-2 gap-3">
                <div className="relative overflow-hidden rounded-[var(--lp-radius)] border border-white/15 p-4">
                  <IconWatermark icon="book" onDark className="-bottom-4 -right-4 size-20" />
                  <p className="relative text-4xl font-semibold leading-none tabular-nums">{mine.length}</p>
                  <p className="text-xs text-white/60">Your submissions</p>
                </div>
                <div className="relative overflow-hidden rounded-[var(--lp-radius)] border border-white/15 p-4">
                  <IconWatermark icon="chart" onDark className="-bottom-4 -right-4 size-20" />
                  <p className="relative text-4xl font-semibold leading-none tabular-nums">{mine.reduce((sum, item) => sum + item.installationCount, 0)}</p>
                  <p className="text-xs text-white/60">Total installs</p>
                </div>
              </div>
            </div>

            <form onSubmit={submit} className="grid gap-4 p-7 md:grid-cols-2">
              <label className="text-sm font-semibold">Template name
                <input className="lp-input mt-1.5" name="name" placeholder="Engineering launch plan" required />
              </label>
              <label className="text-sm font-semibold">Category
                <input className="lp-input mt-1.5" name="category" placeholder="Engineering" required />
              </label>
              <label className="text-sm font-semibold md:col-span-2">Description
                <textarea className="lp-input mt-1.5 min-h-24" name="description" placeholder="What this template helps a team achieve…" required />
              </label>
              <label className="text-sm font-semibold">First step
                <input className="lp-input mt-1.5" name="stepTitle" placeholder="Meet your manager" required />
              </label>
              <label className="text-sm font-semibold">Due after
                <div className="relative">
                  <input className="lp-input mt-1.5" name="dueOffsetDays" type="number" min="0" defaultValue="0" required />
                  <span className="pointer-events-none absolute right-3 top-4 text-xs text-[var(--lp-ink-muted)]">days</span>
                </div>
              </label>
              <label className="text-sm font-semibold md:col-span-2">Instructions
                <input className="lp-input mt-1.5" name="instructions" placeholder="Explain what success looks like" required />
              </label>

              <fieldset className="md:col-span-2">
                <legend className="text-sm font-semibold">Distribution</legend>
                <div className="mt-2 grid grid-cols-2 gap-3">
                  <button type="button" onClick={() => setPaid(false)} className={`rounded-[var(--lp-radius)] border p-4 text-left ${!paid ? "border-[var(--lp-accent)] bg-[var(--lp-paper)]" : "border-[var(--lp-border)]"}`}>
                    <span className="block font-semibold">Free</span>
                    <span className="text-xs text-[var(--lp-ink-muted)]">Anyone can install</span>
                  </button>
                  <button type="button" onClick={() => setPaid(true)} className={`rounded-[var(--lp-radius)] border p-4 text-left ${paid ? "border-[var(--lp-accent)] bg-[var(--lp-paper)]" : "border-[var(--lp-border)]"}`}>
                    <span className="block font-semibold">Paid</span>
                    <span className="text-xs text-[var(--lp-ink-muted)]">Sell a lifetime copy</span>
                  </button>
                </div>
              </fieldset>

              {paid ? (
                <>
                  <label className="text-sm font-semibold">Price
                    <input className="lp-input mt-1.5" name="price" type="number" min="1" step=".01" placeholder="49.00" required />
                  </label>
                  <label className="text-sm font-semibold">Currency
                    <select className="lp-input mt-1.5" name="currency" defaultValue="USD">
                      <option value="USD">USD</option>
                      <option value="GHS">GHS</option>
                      <option value="NGN">NGN</option>
                      <option value="GBP">GBP</option>
                    </select>
                  </label>
                </>
              ) : null}

              <button disabled={pending} className="lp-btn lp-btn--secondary md:w-fit">
                {pending ? "Submitting…" : "Submit for review"}
              </button>
            </form>
          </div>
        </Surface>
      </Reveal>

      {mine.length > 0 ? (
        <Reveal>
          <section>
            <div className="mb-4">
              <p className="lp-eyebrow">Creator workspace</p>
              <h2 className="mt-2 text-2xl font-semibold">Your templates</h2>
            </div>
            <div className="flex gap-3 overflow-x-auto pb-2">
              {mine.map((item) => (
                <Surface key={item.id} className="min-w-72">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-xs font-bold uppercase tracking-wider text-[var(--lp-ink-muted)]">{item.status}</span>
                    <span className="font-semibold">{money(item.priceCents, item.currency)}</span>
                  </div>
                  <h3 className="mt-3 text-lg font-semibold">{item.name}</h3>
                  <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{item.installationCount} installs · {item.ratingCount} ratings</p>
                </Surface>
              ))}
            </div>
          </section>
        </Reveal>
      ) : null}

      <section>
        <div className="mb-5">
          <p className="lp-eyebrow">Reviewed library</p>
          <h2 className="mt-2 text-2xl font-semibold">Marketplace templates</h2>
        </div>
        {loading ? <MarketplaceSkeleton /> : items.length === 0 ? (
          <Surface><EmptyState title="No published templates" description="Reviewed templates will appear here." /></Surface>
        ) : (
          <div className="grid gap-5 lg:grid-cols-2">
            {items.slice(page * 6, page * 6 + 6).map((item) => (
              <Reveal key={item.id}>
                <Surface className="h-full">
                  <div className="flex items-start justify-between gap-3">
                    <div><p className="lp-eyebrow">{item.category}</p><h2 className="mt-2 text-xl font-semibold">{item.name}</h2></div>
                    <span className="rounded-full border border-[var(--lp-border)] px-3 py-1 text-sm font-bold">
                      {money(item.priceCents, item.currency)}
                    </span>
                  </div>
                  <p className="mt-3 text-sm leading-6 text-[var(--lp-ink-muted)]">{item.description}</p>
                  <p className="mt-4 text-xs text-[var(--lp-ink-muted)]">
                    v{item.version} · {item.steps.length} steps · {item.installationCount} installs · {item.ratingAverage.toFixed(1)} / 5
                  </p>
                  <div className="mt-5 flex flex-wrap items-center gap-2">
                    <button disabled={pending} onClick={() => install(item)} className="lp-btn lp-btn--primary">
                      {item.priceCents > 0 ? `Buy for ${money(item.priceCents, item.currency)}` : "Install free"}
                    </button>
                    <span className="ml-auto text-xs text-[var(--lp-ink-muted)]">Rate</span>
                    {[1, 2, 3, 4, 5].map((score) => (
                      <button key={score} disabled={pending} onClick={() => rate(item.id, score)} aria-label={`Rate ${score} stars`} className="grid h-8 w-8 place-items-center rounded-full border border-[var(--lp-border)] text-xs">{score}</button>
                    ))}
                  </div>
                </Surface>
              </Reveal>
            ))}
          </div>
        )}
        {!loading && items.length > 6 ? (
          <div className="mt-5 flex items-center justify-between">
            <p className="text-sm text-[var(--lp-ink-muted)]">
              Page {page + 1} of {Math.ceil(items.length / 6)}
            </p>
            <div className="flex gap-2">
              <button className="lp-btn lp-btn--secondary" disabled={page === 0} onClick={() => setPage((value) => value - 1)}>Previous</button>
              <button className="lp-btn lp-btn--primary" disabled={(page + 1) * 6 >= items.length} onClick={() => setPage((value) => value + 1)}>Next</button>
            </div>
          </div>
        ) : null}
      </section>
    </div>
  );
}
