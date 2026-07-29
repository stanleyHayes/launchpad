"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { KnowledgeDocument, KnowledgeSource } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const sourceOptions = [
  { value: "manual", label: "Manual entry" },
  { value: "url", label: "URL" },
  { value: "upload", label: "Upload" },
  { value: "notion", label: "Notion" },
  { value: "confluence", label: "Confluence" },
  { value: "google_drive", label: "Google Drive" },
  { value: "github", label: "GitHub" },
  { value: "sharepoint", label: "SharePoint" },
  { value: "wiki", label: "Wiki" },
];

const statusBadgeClass: Record<string, string> = {
  draft: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
  approved: "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]",
  indexed: "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  archived: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)] line-through",
};

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function KnowledgePage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [documents, setDocuments] = useState<KnowledgeDocument[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const items = await getClient().listKnowledgeDocuments();
          if (isStale?.()) return;
          setDocuments(items);
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load knowledge documents");
          setLoaded(true);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial load only
  }, [router]);

  function handleActionError(err: unknown, fallback: string) {
    if (err instanceof ApiError && err.status === 401) {
      clearSession();
      router.replace("/login");
      return;
    }
    setError(err instanceof ApiError ? err.message : fallback);
  }

  function onCreate(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);

    startTransition(() => {
      void (async () => {
        try {
          await getClient().createKnowledgeDocument({
            title: formString(form, "title"),
            source: (formString(form, "source") || "manual") as KnowledgeSource,
            uri: formString(form, "uri") || undefined,
            body: formString(form, "body") || undefined,
            accessScope: (formString(form, "accessScope") || "organization") as "organization" | "restricted",
          });
          formEl.reset();
          setMessage("Document created as draft — approve it before indexing");
          reload();
        } catch (err) {
          handleActionError(err, "Unable to create knowledge document");
        }
      })();
    });
  }

  function onAction(
    document: KnowledgeDocument,
    action: "approve" | "index" | "archive" | "sync",
  ) {
    setError(null);
    setMessage(null);
    setBusyId(document.id);

    void (async () => {
      try {
        const client = getClient();
        if (action === "approve") {
          await client.approveKnowledgeDocument(document.id);
        } else if (action === "index") {
          await client.indexKnowledgeDocument(document.id);
        } else if (action === "archive") {
          await client.archiveKnowledgeDocument(document.id);
        } else {
          await client.syncKnowledgeDocument(document.id);
        }
        setMessage(
          action === "approve"
            ? `"${document.title}" approved`
            : action === "index"
              ? `"${document.title}" handed to the AI index`
              : action === "archive"
                ? `"${document.title}" archived`
                : `"${document.title}" synchronized`,
        );
        reload();
      } catch (err) {
        handleActionError(err, `Unable to ${action} "${document.title}"`);
      } finally {
        setBusyId(null);
      }
    })();
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Operations"
          title="Knowledge"
          description="Curate the documents the AI assistant answers from. Approval is required before anything reaches the index."
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

      <Reveal delay={1}>
        <Surface>
          <h2 className="text-lg font-semibold">Add document</h2>
          <form className="mt-4 grid gap-3" onSubmit={onCreate}>
            <input className="lp-input" name="title" placeholder="Title" required />
            <div className="grid gap-3 sm:grid-cols-2">
              <Select className="lp-input" name="source" defaultValue="manual">
                {sourceOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </Select>
              <Select className="lp-input" name="accessScope" defaultValue="organization">
                <option value="organization">Whole organization</option>
                <option value="restricted">Managers only</option>
              </Select>
            </div>
            <input
              className="lp-input"
              name="uri"
              type="url"
              placeholder="Source URL (optional — for linked sources)"
            />
            <textarea
              className="lp-input min-h-24 resize-y"
              name="body"
              placeholder="Content (optional — for manually entered text)"
            />
            <div>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Create draft
              </button>
            </div>
          </form>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Documents</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && !loaded ? "Loading…" : `${documents.length} documents`}
            </p>
          </div>
          {loaded && documents.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No knowledge documents yet"
                description="Add policies, guides, and links — the assistant cites these when answering employees."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {documents.map((document) => (
                <li key={document.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">{document.title}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {document.source.replace("_", " ")} ·{" "}
                        {document.accessScope === "restricted"
                          ? "Managers only"
                          : "Whole organization"}
                        {" · "}v{document.version}
                      </p>
                      {document.uri ? (
                        <a
                          href={document.uri}
                          target="_blank"
                          rel="noreferrer"
                          className="mt-1 block text-sm font-medium text-[var(--lp-brand)] underline-offset-2 hover:underline"
                        >
                          {document.uri}
                        </a>
                      ) : null}
                      {document.lastSyncedAt ? (
                        <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                          Last synced {new Date(document.lastSyncedAt).toLocaleString()}
                        </p>
                      ) : null}
                      {document.syncError ? (
                        <p className="mt-1 text-xs text-[var(--lp-danger)]">{document.syncError}</p>
                      ) : null}
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className={statusBadgeClass[document.status] ?? statusBadgeClass.draft}>
                        {document.status}
                      </span>
                      {document.status === "draft" ? (
                        <button
                          type="button"
                          disabled={busyId === document.id}
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            onAction(document, "approve");
                          }}
                        >
                          Approve
                        </button>
                      ) : null}
                      {document.status === "approved" ? (
                        <button
                          type="button"
                          disabled={busyId === document.id}
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            onAction(document, "index");
                          }}
                        >
                          Index
                        </button>
                      ) : null}
                      {document.status !== "archived" ? (
                        <button
                          type="button"
                          disabled={busyId === document.id}
                          className="lp-btn lp-btn--ghost"
                          onClick={() => {
                            onAction(document, "archive");
                          }}
                        >
                          Archive
                        </button>
                      ) : null}
                      {document.uri && !["manual", "upload"].includes(document.source) ? (
                        <button
                          type="button"
                          disabled={busyId === document.id}
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            onAction(document, "sync");
                          }}
                        >
                          Sync
                        </button>
                      ) : null}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>
    </div>
  );
}
