"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import type { Delivery, JobStatus, StorageOverview } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, MetricCard, PageHeader, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession } from "@/lib/session";

export default function JobsPage() {
  const router = useRouter();
  const [items, setItems] = useState<JobStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [storage, setStorage] = useState<StorageOverview | null>(null);

  const load = useCallback(async () => {
    try {
      const [jobs, deliveryItems, storageOverview] = await Promise.all([
        getClient().listPlatformJobs(),
        getClient().listPlatformDeliveries(),
        getClient().getPlatformStorageOverview(),
      ]);
      setItems(jobs);
      setDeliveries(deliveryItems);
      setStorage(storageOverview);
      setError(null);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setError(cause instanceof ApiError ? cause.message : "Unable to load scheduled jobs");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function run(name: string) {
    setRunning(name);
    setError(null);
    try {
      setItems(await getClient().runPlatformJob(name));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : `Unable to run ${name}`);
    } finally {
      setRunning(null);
    }
  }

  async function retryDelivery(id: string) {
    setRunning(id);
    try {
      await getClient().retryPlatformDelivery(id);
      await load();
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Unable to retry delivery");
    } finally {
      setRunning(null);
    }
  }

  return (
    <div className="space-y-8">
      <PageHeader
        eyebrow="Operations"
        title="Scheduled jobs"
        description="Inspect background sweep health and trigger a controlled retry. Every manual run is audited."
      />
      <section className="grid gap-4 sm:grid-cols-3">
        <MetricCard icon="inbox" label="Database objects" value={storage?.objects.toLocaleString() ?? "—"} accent="#3b82f6" />
        <MetricCard icon="chart" label="Storage" value={storage ? `${(storage.storageSizeBytes / 1_048_576).toFixed(1)} MB` : "—"} accent="#0f766e" />
        <MetricCard icon="settings" label="Indexes" value={storage ? `${(storage.indexSizeBytes / 1_048_576).toFixed(1)} MB` : "—"} accent="#8b5cf6" />
      </section>
      {error ? <p role="alert" className="text-[var(--lp-danger)]">{error}</p> : null}
      <Surface className="overflow-hidden p-0">
        {loading ? <p className="p-5 text-sm text-[var(--lp-ink-muted)]">Loading jobs…</p> : null}
        {!loading && items.length === 0 ? (
          <div className="p-5"><EmptyState dense title="No jobs registered" description="Registered scheduler sweeps appear here." /></div>
        ) : (
          <ul className="divide-y divide-[var(--lp-border)]">
            {items.map((item) => (
              <li key={item.name} className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
                <div>
                  <p className="font-semibold">{item.name.replaceAll("_", " ")}</p>
                  <p className="text-sm text-[var(--lp-ink-muted)]">
                    {item.runCount} runs · {item.failureCount} failures
                    {item.lastCompletedAt ? ` · last completed ${new Date(item.lastCompletedAt).toLocaleString()}` : ""}
                  </p>
                  {item.lastError ? <p className="mt-1 text-sm text-[var(--lp-danger)]">{item.lastError}</p> : null}
                </div>
                <button
                  type="button"
                  className="lp-btn lp-btn--secondary"
                  disabled={item.running || running === item.name}
                  onClick={() => { void run(item.name); }}
                >
                  {item.running || running === item.name ? "Running…" : "Run now"}
                </button>
              </li>
            ))}
          </ul>
        )}
      </Surface>
      <Surface className="overflow-hidden p-0">
        <div className="border-b border-[var(--lp-border)] px-5 py-4">
          <h2 className="text-lg font-semibold">Outbound delivery log</h2>
          <p className="text-sm text-[var(--lp-ink-muted)]">
            {deliveries.filter((item) => item.status === "dead_letter").length} dead-letter deliveries
          </p>
        </div>
        <ul className="divide-y divide-[var(--lp-border)]">
          {deliveries.map((delivery) => (
            <li key={delivery.id} className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
              <div>
                <p className="font-semibold">{delivery.channel.replaceAll("_", " ")} · {delivery.status}</p>
                <p className="text-sm text-[var(--lp-ink-muted)]">Org {delivery.organizationId} · {delivery.attempts} attempts</p>
                {delivery.lastError ? <p className="mt-1 text-sm text-[var(--lp-danger)]">{delivery.lastError}</p> : null}
              </div>
              {delivery.status !== "delivered" ? (
                <button className="lp-btn lp-btn--secondary" disabled={running === delivery.id} onClick={() => { void retryDelivery(delivery.id); }}>
                  {running === delivery.id ? "Retrying…" : "Retry now"}
                </button>
              ) : null}
            </li>
          ))}
        </ul>
      </Surface>
    </div>
  );
}
