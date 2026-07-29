"use client";

import Link from "next/link";
import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useParams, useRouter } from "next/navigation";
import type {
  Assessment,
  AssessmentAttempt,
  AssessmentQuestion,
  AssessmentQuestionInput,
  AssessmentQuestionType,
} from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const questionTypeOptions: { value: AssessmentQuestionType; label: string }[] = [
  { value: "single_choice", label: "Single choice" },
  { value: "multiple_choice", label: "Multiple choice" },
  { value: "true_false", label: "True or false" },
  { value: "short_answer", label: "Short answer" },
];

// Editor-side question draft. acceptedAnswers is edited one per line and
// split on save.
type QuestionDraft = {
  id?: string;
  type: AssessmentQuestionType;
  text: string;
  options: string[];
  correctOptions: number[];
  acceptedAnswers: string;
  points: number;
};

function toDraft(question: AssessmentQuestion): QuestionDraft {
  return {
    id: question.id,
    type: question.type,
    text: question.text,
    options: question.options ?? (question.type === "true_false" ? ["True", "False"] : ["", ""]),
    correctOptions: question.correctOptions ?? [],
    acceptedAnswers: (question.acceptedAnswers ?? []).join("\n"),
    points: question.points,
  };
}

function newQuestion(): QuestionDraft {
  return {
    type: "single_choice",
    text: "",
    options: ["", ""],
    correctOptions: [0],
    acceptedAnswers: "",
    points: 1,
  };
}

function toInput(question: QuestionDraft): AssessmentQuestionInput {
  const input: AssessmentQuestionInput = {
    id: question.id,
    type: question.type,
    text: question.text.trim(),
    points: question.points,
  };

  if (question.type === "short_answer") {
    input.acceptedAnswers = question.acceptedAnswers
      .split("\n")
      .map((answer) => answer.trim())
      .filter((answer) => answer !== "");
    return input;
  }

  if (question.type === "true_false") {
    input.options = ["True", "False"];
    input.correctOptions = question.correctOptions.slice(0, 1);
    return input;
  }

  input.options = question.options.map((option) => option.trim());
  input.correctOptions =
    question.type === "single_choice"
      ? question.correctOptions.slice(0, 1)
      : [...question.correctOptions].sort((a, b) => a - b);
  return input;
}

function validateDrafts(questions: QuestionDraft[]): string | null {
  if (questions.length === 0) {
    return "Add at least one question.";
  }

  for (const [index, question] of questions.entries()) {
    const label = `Question ${String(index + 1)}`;
    if (question.text.trim() === "") {
      return `${label}: enter the question text.`;
    }

    if (question.type === "short_answer") {
      if (question.acceptedAnswers.trim() === "") {
        return `${label}: short answers need at least one accepted answer.`;
      }
      continue;
    }

    if (question.type === "true_false") {
      if (question.correctOptions.length !== 1) {
        return `${label}: mark whether True or False is correct.`;
      }
      continue;
    }

    if (question.options.length < 2 || question.options.some((option) => option.trim() === "")) {
      return `${label}: choice questions need at least two non-empty options.`;
    }

    if (
      question.correctOptions.length === 0 ||
      (question.type === "single_choice" && question.correctOptions.length !== 1) ||
      question.correctOptions.some((option) => option >= question.options.length)
    ) {
      return `${label}: mark the correct option${question.type === "multiple_choice" ? "s" : ""}.`;
    }
  }

  return null;
}

export default function AssessmentDetailPage() {
  const router = useRouter();
  const params = useParams<{ assessmentID: string }>();
  const assessmentId = params.assessmentID;
  const [pending, startTransition] = useTransition();
  const [assessment, setAssessment] = useState<Assessment | null>(null);
  const [attempts, setAttempts] = useState<AssessmentAttempt[]>([]);
  const [questions, setQuestions] = useState<QuestionDraft[]>([]);
  const [reviewScores, setReviewScores] = useState<Record<string, string>>({});
  const [reviewNotes, setReviewNotes] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [loaded, attemptItems] = await Promise.all([
            client.getAssessment(assessmentId),
            client.listAssessmentAttempts(assessmentId),
          ]);
          if (isStale?.()) return;
          setAssessment(loaded);
          setAttempts(attemptItems);
          setQuestions(loaded.questions.map(toDraft));
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load assessment");
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
  }, [router, assessmentId]);

  function updateQuestion(index: number, patch: Partial<QuestionDraft>) {
    setQuestions((current) =>
      current.map((question, i) => (i === index ? { ...question, ...patch } : question)),
    );
  }

  function onSave(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const form = new FormData(event.currentTarget);
    const validation = validateDrafts(questions);
    if (validation) {
      setError(validation);
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          await getClient().updateAssessment(assessmentId, {
            title: String(form.get("title") ?? "").trim(),
            description: String(form.get("description") ?? "").trim(),
            questions: questions.map(toInput),
            passingScore: Number(form.get("passingScore") || "70"),
            maxAttempts: Number(form.get("maxAttempts") || "0"),
            randomize: form.get("randomize") === "on",
          });
          setMessage("Draft saved");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to save assessment");
        }
      })();
    });
  }

  function onPublish() {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().publishAssessment(assessmentId);
          setMessage("Assessment published — employees can take it now");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to publish assessment");
        }
      })();
    });
  }

  function onReview(attempt: AssessmentAttempt) {
    setError(null);
    setMessage(null);
    setBusyId(attempt.id);

    const rawScore = reviewScores[attempt.id];
    const score = rawScore === undefined || rawScore === "" ? attempt.score : Number(rawScore);
    const note = (reviewNotes[attempt.id] ?? "").trim();

    void (async () => {
      try {
        await getClient().reviewAssessmentAttempt(assessmentId, attempt.id, {
          score,
          note: note === "" ? undefined : note,
        });
        setMessage(`Attempt #${String(attempt.attemptNumber)} finalized at ${String(score)}%`);
        reload();
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "Unable to finalize attempt");
      } finally {
        setBusyId(null);
      }
    })();
  }

  const isDraft = assessment?.status === "draft";

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Assessment builder"
          title={assessment?.title ?? "Assessment"}
          description={
            assessment
              ? `${assessment.status} · pass at ${String(assessment.passingScore)}% · ${
                  assessment.maxAttempts === 0
                    ? "unlimited attempts"
                    : `${String(assessment.maxAttempts)} attempts`
                }`
              : "Loading…"
          }
          actions={
            <Link href="/assessments" className="text-sm font-semibold text-[var(--lp-accent)]">
              ← All assessments
            </Link>
          }
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

      {assessment ? (
        <Reveal delay={1}>
          <Surface>
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold">
                  {isDraft ? "Edit draft" : "Assessment definition"}
                </h2>
                {!isDraft ? (
                  <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                    Only drafts can be edited. Archive and recreate to change a published
                    assessment.
                  </p>
                ) : null}
              </div>
              {isDraft ? (
                <button
                  type="button"
                  disabled={pending}
                  onClick={onPublish}
                  className="lp-btn lp-btn--primary"
                >
                  Publish
                </button>
              ) : null}
            </div>

            <form onSubmit={onSave} className="mt-4 space-y-4">
              <fieldset disabled={!isDraft || pending} className="space-y-4">
                <div className="grid gap-3 sm:grid-cols-2">
                  <input
                    className="lp-input"
                    name="title"
                    defaultValue={assessment.title}
                    placeholder="Title"
                    required
                  />
                  <input
                    className="lp-input"
                    name="description"
                    defaultValue={assessment.description}
                    placeholder="Description (optional)"
                  />
                  <label className="block text-sm font-semibold">
                    Passing score (%)
                    <input
                      className="lp-input mt-1.5"
                      name="passingScore"
                      type="number"
                      min={0}
                      max={100}
                      defaultValue={assessment.passingScore}
                      required
                    />
                  </label>
                  <label className="block text-sm font-semibold">
                    Max attempts (0 = unlimited)
                    <input
                      className="lp-input mt-1.5"
                      name="maxAttempts"
                      type="number"
                      min={0}
                      defaultValue={assessment.maxAttempts}
                    />
                  </label>
                </div>
                <label className="flex items-center gap-2 text-sm font-semibold">
                  <input type="checkbox" name="randomize" defaultChecked={assessment.randomize} />
                  Randomize question order per attempt
                </label>

                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-semibold">Questions</h3>
                    <button
                      type="button"
                      className="lp-btn lp-btn--secondary"
                      onClick={() => {
                        setQuestions((current) => [...current, newQuestion()]);
                      }}
                    >
                      Add question
                    </button>
                  </div>
                  {questions.map((question, questionIndex) => (
                    <div
                      key={questionIndex}
                      className="space-y-2 rounded-[var(--lp-radius)] border border-[var(--lp-border)] p-3"
                    >
                      <div className="flex flex-wrap gap-2">
                        <select
                          className="lp-input w-auto"
                          value={question.type}
                          onChange={(event) => {
                            const type = event.target.value as AssessmentQuestionType;
                            updateQuestion(questionIndex, {
                              type,
                              options:
                                type === "true_false"
                                  ? ["True", "False"]
                                  : question.options.length >= 2
                                    ? question.options
                                    : ["", ""],
                              correctOptions: [],
                            });
                          }}
                        >
                          {questionTypeOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                        <input
                          className="lp-input min-w-0 flex-1"
                          placeholder={`Question ${String(questionIndex + 1)}`}
                          value={question.text}
                          onChange={(event) => {
                            updateQuestion(questionIndex, { text: event.target.value });
                          }}
                          required
                        />
                        <label className="flex items-center gap-1 text-sm font-semibold">
                          Pts
                          <input
                            className="lp-input w-20"
                            type="number"
                            min={1}
                            value={question.points}
                            onChange={(event) => {
                              updateQuestion(questionIndex, {
                                points: Number(event.target.value || "1"),
                              });
                            }}
                          />
                        </label>
                        <button
                          type="button"
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            setQuestions((current) =>
                              current.filter((_, i) => i !== questionIndex),
                            );
                          }}
                        >
                          Remove
                        </button>
                      </div>

                      {question.type === "short_answer" ? (
                        <label className="block text-sm font-semibold">
                          Accepted answers (one per line, case-insensitive)
                          <textarea
                            className="lp-input mt-1.5 min-h-20 resize-y"
                            value={question.acceptedAnswers}
                            onChange={(event) => {
                              updateQuestion(questionIndex, {
                                acceptedAnswers: event.target.value,
                              });
                            }}
                          />
                        </label>
                      ) : (
                        question.options.map((option, optionIndex) => (
                          <div key={optionIndex} className="flex items-center gap-2">
                            <input
                              type={question.type === "multiple_choice" ? "checkbox" : "radio"}
                              name={`correct-${String(questionIndex)}`}
                              title="Correct answer"
                              checked={question.correctOptions.includes(optionIndex)}
                              onChange={() => {
                                updateQuestion(questionIndex, {
                                  correctOptions:
                                    question.type === "multiple_choice"
                                      ? question.correctOptions.includes(optionIndex)
                                        ? question.correctOptions.filter((i) => i !== optionIndex)
                                        : [...question.correctOptions, optionIndex]
                                      : [optionIndex],
                                });
                              }}
                            />
                            <input
                              className="lp-input"
                              placeholder={`Option ${String(optionIndex + 1)}`}
                              value={option}
                              disabled={question.type === "true_false"}
                              onChange={(event) => {
                                updateQuestion(questionIndex, {
                                  options: question.options.map((current, i) =>
                                    i === optionIndex ? event.target.value : current,
                                  ),
                                });
                              }}
                              required
                            />
                            {question.type !== "true_false" ? (
                              <button
                                type="button"
                                className="lp-btn lp-btn--secondary"
                                disabled={question.options.length <= 2}
                                onClick={() => {
                                  const options = question.options.filter(
                                    (_, i) => i !== optionIndex,
                                  );
                                  updateQuestion(questionIndex, {
                                    options,
                                    correctOptions: question.correctOptions
                                      .filter((i) => i !== optionIndex)
                                      .map((i) => (i > optionIndex ? i - 1 : i)),
                                  });
                                }}
                              >
                                ×
                              </button>
                            ) : null}
                          </div>
                        ))
                      )}

                      {question.type === "single_choice" || question.type === "multiple_choice" ? (
                        <button
                          type="button"
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            updateQuestion(questionIndex, {
                              options: [...question.options, ""],
                            });
                          }}
                        >
                          Add option
                        </button>
                      ) : null}
                    </div>
                  ))}
                </div>
              </fieldset>

              {isDraft ? (
                <button type="submit" disabled={pending} className="lp-btn lp-btn--primary">
                  Save draft
                </button>
              ) : null}
            </form>
          </Surface>
        </Reveal>
      ) : null}

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Attempts</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {attempts.length} attempts · pending_review attempts need a final score from you
            </p>
          </div>
          {attempts.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No attempts yet"
                description="Attempts appear once employees start taking this assessment."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {attempts.map((attempt) => (
                <li key={attempt.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">
                        Employee {attempt.employeeId} · attempt #{attempt.attemptNumber}
                      </p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {attempt.status === "pending_review"
                          ? `Auto score ${String(attempt.score)}% — awaiting review`
                          : `Score ${String(attempt.score)}% · ${attempt.passed ? "passed" : "failed"}`}
                        {" · "}
                        {new Date(attempt.submittedAt).toLocaleString()}
                      </p>
                      {attempt.reviewNote ? (
                        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                          Review note: {attempt.reviewNote}
                        </p>
                      ) : null}
                    </div>
                    {attempt.status === "pending_review" ? (
                      <div className="flex flex-wrap items-center gap-2">
                        <input
                          className="lp-input w-24"
                          type="number"
                          min={0}
                          max={100}
                          placeholder={String(attempt.score)}
                          value={reviewScores[attempt.id] ?? ""}
                          onChange={(event) => {
                            setReviewScores((current) => ({
                              ...current,
                              [attempt.id]: event.target.value,
                            }));
                          }}
                        />
                        <input
                          className="lp-input w-56"
                          placeholder="Review note (optional)"
                          value={reviewNotes[attempt.id] ?? ""}
                          onChange={(event) => {
                            setReviewNotes((current) => ({
                              ...current,
                              [attempt.id]: event.target.value,
                            }));
                          }}
                        />
                        <button
                          type="button"
                          disabled={busyId === attempt.id}
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            onReview(attempt);
                          }}
                        >
                          Finalize score
                        </button>
                      </div>
                    ) : null}
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
