"use client";

import { useEffect, useState, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { AssistantAnswer } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, EmptyState, Icon, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

type Exchange = {
  id: string;
  question: string;
  answer: AssistantAnswer | null;
  error: string | null;
  feedback: boolean | null;
  escalated: boolean;
};

export default function AssistantPage() {
  const router = useRouter();
  const [question, setQuestion] = useState("");
  const [asking, setAsking] = useState(false);
  const [exchanges, setExchanges] = useState<Exchange[]>([]);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
    }
  }, [router]);

  function onAsk(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const asked = question.trim();
    if (!asked || asking) return;

    const id = `q-${String(Date.now())}`;
    setAsking(true);
    setExchanges((prev) => [
      { id, question: asked, answer: null, error: null, feedback: null, escalated: false },
      ...prev,
    ]);
    setQuestion("");

    void (async () => {
      try {
        const answer = await getClient().askAssistant(asked);
        setExchanges((prev) =>
          prev.map((exchange) =>
            exchange.id === id ? { ...exchange, answer } : exchange,
          ),
        );
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          clearSession();
          router.replace("/login");
          return;
        }
        const message =
          err instanceof ApiError && err.status === 429
            ? "You're asking too quickly — the assistant is limited to 10 questions per minute. Wait a moment and try again."
            : err instanceof ApiError
              ? err.message
              : "Unable to get an answer right now";
        setExchanges((prev) =>
          prev.map((exchange) =>
            exchange.id === id ? { ...exchange, error: message } : exchange,
          ),
        );
      } finally {
        setAsking(false);
      }
    })();
  }

  function onFeedback(exchangeId: string, interactionId: string, helpful: boolean) {
    setExchanges((prev) =>
      prev.map((exchange) =>
        exchange.id === exchangeId ? { ...exchange, feedback: helpful } : exchange,
      ),
    );

    void (async () => {
      try {
        await getClient().submitAssistantFeedback(interactionId, { helpful });
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          clearSession();
          router.replace("/login");
          return;
        }
        // Revert the optimistic mark so the user can try again.
        setExchanges((prev) =>
          prev.map((exchange) =>
            exchange.id === exchangeId ? { ...exchange, feedback: null } : exchange,
          ),
        );
      }
    })();
  }

  function onEscalate(exchangeId: string, asked: string) {
    setExchanges((prev) =>
      prev.map((exchange) =>
        exchange.id === exchangeId ? { ...exchange, escalated: true } : exchange,
      ),
    );

    void (async () => {
      try {
        // Pre-fill the unanswered question as the ticket message.
        await getClient().createSupportTicket({
          subject: `Unanswered assistant question: ${asked.slice(0, 100)}`,
          body: `The assistant could not find a reliable source for this question:\n\n${asked}`,
          category: "other",
        });
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          clearSession();
          router.replace("/login");
          return;
        }
        // Revert the optimistic mark so the user can try again.
        setExchanges((prev) =>
          prev.map((exchange) =>
            exchange.id === exchangeId ? { ...exchange, escalated: false } : exchange,
          ),
        );
      }
    })();
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <Surface className="overflow-hidden">
          <PageHeader
            eyebrow="Help"
            title="Assistant"
            description="Ask anything about your onboarding — answers are grounded in your organization's approved documents, with sources cited."
          />
        </Surface>
      </Reveal>

      <Reveal delay={1}>
        <Surface>
          <form onSubmit={onAsk} className="flex flex-col gap-3 sm:flex-row">
            <label className="sr-only" htmlFor="assistant-question">
              Ask the assistant a question
            </label>
            <input
              id="assistant-question"
              className="lp-input flex-1"
              value={question}
              onChange={(event) => {
                setQuestion(event.target.value);
              }}
              placeholder="e.g. How do I enroll in benefits?"
              maxLength={2000}
              disabled={asking}
            />
            <Button type="submit" disabled={asking || !question.trim()}>
              {asking ? "Thinking…" : "Ask"}
            </Button>
          </form>
          <p className="mt-2 text-xs text-[var(--lp-ink-muted)]">
            Answers come only from approved knowledge documents — if nothing
            reliable matches, the assistant will say so. Limited to 10 questions
            per minute.
          </p>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <div className="space-y-4">
          {exchanges.length === 0 ? (
            <Surface>
              <EmptyState
                title="No questions yet"
                description="Ask about policies, equipment, benefits, or anything in your onboarding documents."
              />
            </Surface>
          ) : (
            exchanges.map((exchange) => (
              <Surface key={exchange.id}>
                <p className="text-sm font-semibold text-[var(--lp-ink-muted)]">
                  You asked
                </p>
                <p className="mt-1 font-medium">{exchange.question}</p>

                <div className="mt-4 border-t border-[var(--lp-border)] pt-4">
                  {exchange.error ? (
                    <p
                      className="rounded-[var(--lp-radius)] bg-[var(--lp-danger)]/10 px-3 py-2 text-sm text-[var(--lp-danger)]"
                      role="alert"
                    >
                      {exchange.error}
                    </p>
                  ) : exchange.answer === null ? (
                    <p className="flex items-center gap-2 text-sm text-[var(--lp-ink-muted)]">
                      <Icon name="sparkles" className="h-4 w-4 animate-pulse text-[var(--lp-brand)]" />
                      Finding an answer…
                    </p>
                  ) : exchange.answer.refused ? (
                    <div className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)]/[0.06] px-3 py-2">
                      <p className="text-sm font-semibold">No reliable source found</p>
                      <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                        {exchange.answer.text}
                      </p>
                      <p className="mt-2 text-xs text-[var(--lp-ink-muted)]">
                        The assistant only answers from approved documents rather
                        than guessing. Try rephrasing, or ask your manager or HR
                        team directly.
                      </p>
                      <div className="mt-3">
                        <button
                          type="button"
                          className="lp-btn lp-btn--secondary"
                          disabled={exchange.escalated}
                          onClick={() => {
                            onEscalate(exchange.id, exchange.question);
                          }}
                        >
                          {exchange.escalated ? "Ticket created — see Support" : "Create support ticket"}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <p className="whitespace-pre-wrap text-sm leading-relaxed">
                        {exchange.answer.text}
                      </p>

                      {exchange.answer.citations.length > 0 ? (
                        <div className="mt-4">
                          <p className="lp-eyebrow">Sources</p>
                          <ul className="mt-2 space-y-1.5">
                            {exchange.answer.citations.map((citation, index) => (
                              <li key={`${citation.documentId}-${String(index)}`} className="text-sm">
                                <span className="font-semibold text-[var(--lp-brand)]">
                                  [{index + 1}]
                                </span>{" "}
                                {citation.documentUri ? (
                                  <a
                                    href={citation.documentUri}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="font-medium text-[var(--lp-brand)] underline-offset-2 hover:underline"
                                  >
                                    {citation.documentTitle}
                                  </a>
                                ) : (
                                  <span className="font-medium">{citation.documentTitle}</span>
                                )}
                                {citation.snippet ? (
                                  <span className="text-[var(--lp-ink-muted)]">
                                    {" "}— {citation.snippet}
                                  </span>
                                ) : null}
                              </li>
                            ))}
                          </ul>
                        </div>
                      ) : null}

                      <div className="mt-4 flex items-center gap-2">
                        <span className="text-xs text-[var(--lp-ink-muted)]">
                          Was this helpful?
                        </span>
                        <button
                          type="button"
                          className={`lp-btn lp-btn--secondary ${exchange.feedback === true ? "ring-2 ring-[var(--lp-brand)]" : ""}`}
                          disabled={exchange.feedback !== null}
                          onClick={() => {
                            onFeedback(exchange.id, exchange.answer!.interactionId, true);
                          }}
                        >
                          {exchange.feedback === true ? "Thanks!" : "Yes"}
                        </button>
                        <button
                          type="button"
                          className={`lp-btn lp-btn--ghost ${exchange.feedback === false ? "ring-2 ring-[var(--lp-brand)]" : ""}`}
                          disabled={exchange.feedback !== null}
                          onClick={() => {
                            onFeedback(exchange.id, exchange.answer!.interactionId, false);
                          }}
                        >
                          {exchange.feedback === false ? "Noted" : "No"}
                        </button>
                      </div>
                    </>
                  )}
                </div>
              </Surface>
            ))
          )}
        </div>
      </Reveal>
    </div>
  );
}
