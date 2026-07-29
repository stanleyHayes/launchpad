"use client";

import { useEffect, useState, useTransition, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import type { CMSPage } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";
import { SiteInformationEditor } from "./site-information-editor";

export default function CMSPagesPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [pages, setPages] = useState<CMSPage[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [slug, setSlug] = useState("");
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [body, setBody] = useState("");
  const [contentType, setContentType] = useState<"page" | "blog" | "faq" | "legal">("page");
  const [navLabel, setNavLabel] = useState("");
  const [navOrder, setNavOrder] = useState("0");

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const pageItems = await getClient().listPlatformCMSPages();
          if (isStale?.()) return;
          setPages(pageItems);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load CMS pages");
        }
      })();
    });
  }

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    let stale = false;
    reload(() => stale);
    return () => {
      stale = true;
    };
  }, [router]);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createPlatformCMSPage({
            slug, title, summary, body, contentType,
            navLabel: navLabel || undefined, navOrder: Number(navOrder || "0"),
          });
          setSlug("");
          setTitle("");
          setSummary("");
          setBody("");
          setNavLabel("");
          setNavOrder("0");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create page");
        }
      })();
    });
  }

  async function onPublish(pageId: string) {
    startTransition(() => {
      void (async () => {
        try {
          await getClient().publishPlatformCMSPage(pageId);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to publish page");
        }
      })();
    });
  }

  async function onSchedule(pageId: string) {
    const value = window.prompt("Publish at (for example 2026-08-01T09:00:00+00:00)");
    if (!value) return;
    const publishAt = new Date(value);
    if (Number.isNaN(publishAt.getTime())) {
      setError("Enter a valid publication date and time");
      return;
    }
    startTransition(() => {
      void (async () => {
        try {
          await getClient().schedulePlatformCMSPage(pageId, publishAt.toISOString());
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to schedule page");
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Content"
            title="CMS pages"
            description="Draft and publish marketing pages without redeploying the site."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <SiteInformationEditor pages={pages} onSaved={() => reload()} />
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2 className="text-lg font-semibold">New draft</h2>
            <form className="mt-4 grid gap-3" onSubmit={onCreate}>
              <input
                className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                placeholder="slug (e.g. pricing)"
                value={slug}
                onChange={(event) => setSlug(event.target.value)}
                required
              />
              <Select
                className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                value={contentType}
                onChange={(event) => setContentType(event.target.value as typeof contentType)}
              >
                <option value="page">Page</option>
                <option value="blog">Blog post</option>
                <option value="faq">FAQ</option>
                <option value="legal">Legal page</option>
              </Select>
              <div className="grid gap-3 sm:grid-cols-2">
                <input className="lp-input" placeholder="Navigation label (optional)" value={navLabel} onChange={(event) => setNavLabel(event.target.value)} />
                <input className="lp-input" type="number" min="0" placeholder="Navigation order" value={navOrder} onChange={(event) => setNavOrder(event.target.value)} />
              </div>
              <input
                className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                placeholder="Title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                required
              />
              <input
                className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                placeholder="Summary"
                value={summary}
                onChange={(event) => setSummary(event.target.value)}
              />
              <textarea
                className="min-h-32 rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                placeholder="Body"
                value={body}
                onChange={(event) => setBody(event.target.value)}
                required
              />
              <button
                type="submit"
                disabled={pending}
                className="justify-self-start rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Create draft
              </button>
            </form>
          </Surface>
        </Reveal>

        <Reveal delay={3}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">All pages</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && pages.length === 0 ? "Loading…" : `${pages.length} pages`}
              </p>
            </div>
            {pages.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No CMS pages"
                  description="Create a draft above, then publish it for the public API."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {pages.filter((page) => page.slug !== "site-information").map((page) => (
                  <li key={page.id} className="flex flex-wrap items-start justify-between gap-3 px-5 py-4">
                    <div>
                      <p className="font-medium">{page.title}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        /{page.slug} · {page.contentType} · {page.status}
                      </p>
                      {page.navLabel ? <p className="text-xs text-[var(--lp-ink-muted)]">Navigation: {page.navLabel} · position {page.navOrder ?? 0}</p> : null}
                      {page.summary ? (
                        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">{page.summary}</p>
                      ) : null}
                    </div>
                    {page.status === "draft" ? (
                      <div className="flex gap-2">
                        <button type="button" disabled={pending} onClick={() => { void onSchedule(page.id); }} className="lp-btn lp-btn--secondary">
                          Schedule
                        </button>
                        <button type="button" disabled={pending} onClick={() => { void onPublish(page.id); }} className="lp-btn lp-btn--secondary">
                          Publish
                        </button>
                      </div>
                    ) : (
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {page.status === "scheduled" && page.scheduledAt
                          ? `Scheduled ${new Date(page.scheduledAt).toLocaleString()}`
                          : "Live"}
                      </p>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
      );
}
