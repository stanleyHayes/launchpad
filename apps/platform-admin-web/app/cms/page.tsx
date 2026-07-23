"use client";

import { useEffect, useState, useTransition, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import type { CMSPage } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function CMSPagesPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [pages, setPages] = useState<CMSPage[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [slug, setSlug] = useState("");
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [body, setBody] = useState("");

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setPages(await getClient().listPlatformCMSPages());
          setError(null);
        } catch (err) {
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
    reload();
  }, [router]);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createPlatformCMSPage({ slug, title, summary, body });
          setSlug("");
          setTitle("");
          setSummary("");
          setBody("");
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

  return (
    <PlatformShell>
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

        <Reveal delay={2}>
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
                {pages.map((page) => (
                  <li key={page.id} className="flex flex-wrap items-start justify-between gap-3 px-5 py-4">
                    <div>
                      <p className="font-medium">{page.title}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        /{page.slug} · {page.status}
                      </p>
                      {page.summary ? (
                        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">{page.summary}</p>
                      ) : null}
                    </div>
                    {page.status === "draft" ? (
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          void onPublish(page.id);
                        }}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-medium disabled:opacity-60"
                      >
                        Publish
                      </button>
                    ) : (
                      <p className="text-sm text-[var(--lp-ink-muted)]">Live</p>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </PlatformShell>
  );
}
