"use client";

import Link from "next/link";
import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useParams, useRouter } from "next/navigation";
import type {
  AddJourneyStepRequest,
  Assessment,
  Department,
  JourneyStep,
  JourneyTemplate,
  JourneyVersionSummary,
} from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

type QuizQuestionDraft = {
  text: string;
  options: string[];
  correctOption: number;
};

function newQuizQuestion(): QuizQuestionDraft {
  return { text: "", options: ["", ""], correctOption: 0 };
}

export default function JourneyDetailPage() {
  const router = useRouter();
  const params = useParams<{ journeyID: string }>();
  const journeyId = params.journeyID;
  const [pending, startTransition] = useTransition();
  const [journey, setJourney] = useState<JourneyTemplate | null>(null);
  const [steps, setSteps] = useState<JourneyStep[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [versions, setVersions] = useState<JourneyVersionSummary[]>([]);
  const [stepType, setStepType] = useState("document");
  const [quizQuestions, setQuizQuestions] = useState<QuizQuestionDraft[]>([]);
  const [assessments, setAssessments] = useState<Assessment[]>([]);
  const [stepAssessmentId, setStepAssessmentId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [template, stepItems, departmentItems, versionItems, assessmentItems] =
            await Promise.all([
              client.getJourney(journeyId),
              client.listJourneySteps(journeyId),
              client.listDepartments(),
              client.listJourneyVersions(journeyId),
              client.listAssessments(),
            ]);
          if (isStale?.()) return;
          setJourney(template);
          setSteps(stepItems);
          setDepartments(departmentItems);
          setVersions(versionItems);
          setAssessments(assessmentItems.filter((item) => item.status === "published"));
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load journey");
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
  }, [router, journeyId]);

  function updateQuizQuestion(index: number, patch: Partial<QuizQuestionDraft>) {
    setQuizQuestions((current) =>
      current.map((question, i) => (i === index ? { ...question, ...patch } : question)),
    );
  }

  function onAddStep(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const formEl = event.currentTarget;
    const form = new FormData(formEl);

    const payload: AddJourneyStepRequest = {
      stepType,
      title: formString(form, "title"),
      instructions: formString(form, "instructions"),
      dueOffsetDays: Number(formString(form, "dueOffsetDays") || "0"),
      businessDays: form.get("businessDays") === "on",
      stage: formString(form, "stage") || undefined,
      parallelGroup: formString(form, "parallelGroup") || undefined,
      prerequisiteStepIds: formString(form, "prerequisiteStepIds").split(",").map((value) => value.trim()).filter(Boolean),
      locale: formString(form, "locale") || undefined,
    };

    if (stepType === "quiz") {
      const questions = quizQuestions
        .map((question, index) => ({
          id: `q${String(index + 1)}`,
          text: question.text.trim(),
          options: question.options.map((option) => option.trim()),
          correctOption: question.correctOption,
        }))
        .filter((question) => question.text !== "");

      const invalid =
        questions.length === 0 ||
        questions.some(
          (question) =>
            question.options.length < 2 ||
            question.options.some((option) => option === "") ||
            question.correctOption >= question.options.length,
        );

      if (invalid) {
        setError(
          "Quiz steps need at least one question, each with two or more options and a marked correct answer.",
        );
        return;
      }

      payload.config = { questions };
    }

    if (stepType === "assessment") {
      if (stepAssessmentId === "") {
        setError("Assessment steps need a published assessment — create one under Assessments.");
        return;
      }

      payload.config = { assessmentId: stepAssessmentId };
    }

    startTransition(() => {
      void (async () => {
        try {
          await getClient().addJourneyStep(journeyId, payload);
          formEl.reset();
          setQuizQuestions([]);
          setStepAssessmentId("");
          setMessage("Step added");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to add step");
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
          await getClient().publishJourney(journeyId);
          setMessage("Journey published");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to publish journey");
        }
      })();
    });
  }

  function onDeleteStep(stepId: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().deleteJourneyStep(journeyId, stepId);
          setMessage("Step deleted");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to delete step");
        }
      })();
    });
  }

  function onCreateVersion() {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createJourneyVersion(journeyId);
          setMessage("New draft version created — edit it, then publish");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create new version");
        }
      })();
    });
  }

  function onClone() {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          const clone = await getClient().cloneJourney(journeyId);
          router.push(`/journeys/${clone.id}`);
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to clone journey");
        }
      })();
    });
  }

  function onRollback(version: number) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().rollbackJourney(journeyId, version);
          setMessage(`Rolled back to version ${String(version)}`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to roll back journey");
        }
      })();
    });
  }

  function onAssignDepartment(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const startsAt = formString(form, "startsAt");
    startTransition(() => {
      void (async () => {
        try {
          const result = await getClient().assignJourneyToDepartment({
            departmentId: formString(form, "departmentId"),
            journeyTemplateId: journeyId,
            startsAt: startsAt === "" ? undefined : startsAt,
          });
          formEl.reset();
          setMessage(
            `Assigned to ${String(result.assigned)} of ${String(result.employees)} employees` +
              (result.skipped > 0 ? ` (${String(result.skipped)} already had it)` : "") +
              ".",
          );
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to assign journey");
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Journey detail"
            title={journey?.name ?? "Journey"}
            description={
              journey
                ? `${journey.status} · version ${String(journey.currentVersion)}`
                : "Loading…"
            }
            actions={
              <Link href="/journeys" className="text-sm font-semibold text-[var(--lp-accent)]">
                ← All journeys
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

        {journey?.status === "draft" ? (
          <Reveal delay={1}>
            <Surface>
              <h2 className="text-lg font-semibold">Add step</h2>
              <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                Build the journey from content, submissions, sessions, approvals, requests, and assessments.
              </p>
              <form onSubmit={onAddStep} className="mt-4 grid gap-3 md:grid-cols-2">
                <select
                  className="lp-input"
                  name="stepType"
                  value={stepType}
                  onChange={(event) => {
                    setStepType(event.target.value);
                  }}
                  required
                >
                  <option value="document">Document</option>
                  <option value="information">Information page</option>
                  <option value="policy_acknowledgement">Policy acknowledgement</option>
                  <option value="video">Video</option>
                  <option value="external_course">External course</option>
                  <option value="quiz">Quiz</option>
                  <option value="survey">Survey</option>
                  <option value="file_submission">File submission</option>
                  <option value="text_submission">Text submission</option>
                  <option value="coding_exercise">Coding exercise</option>
                  <option value="assessment">Assessment</option>
                  <option value="task">Task</option>
                  <option value="shadowing_session">Shadowing session</option>
                  <option value="checklist">Checklist</option>
                  <option value="approval">Approval</option>
                  <option value="equipment_request">Equipment request</option>
                  <option value="access_request">Access request</option>
                  <option value="integration_action">Automated integration action</option>
                  <option value="manager_feedback">Manager feedback</option>
                  <option value="employee_reflection">Employee reflection</option>
                  <option value="certification">Certification</option>
                  <option value="meeting">Meeting</option>
                </select>
                <input className="lp-input" name="title" placeholder="Step title" required />
                <input
                  className="lp-input md:col-span-2"
                  name="instructions"
                  placeholder="Instructions"
                />
                <input
                  className="lp-input"
                  name="dueOffsetDays"
                  type="number"
                  min={0}
                  defaultValue={0}
                  placeholder="Due offset days"
                />
                <input className="lp-input" name="stage" placeholder="Stage (optional)" />
                <input className="lp-input" name="parallelGroup" placeholder="Parallel group (optional)" />
                <input className="lp-input md:col-span-2" name="prerequisiteStepIds" placeholder="Prerequisite step IDs, comma-separated" />
                <input className="lp-input" name="locale" placeholder="Locale, e.g. en-GH" defaultValue="en" />
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" name="businessDays" /> Count due offset in business days
                </label>
                {stepType === "assessment" ? (
                  <div className="space-y-2 rounded-[var(--lp-radius)] border border-[var(--lp-border)] p-4 md:col-span-2">
                    <h3 className="text-sm font-semibold">Linked assessment</h3>
                    {assessments.length === 0 ? (
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        No published assessments yet — create and publish one under Assessments
                        first.
                      </p>
                    ) : (
                      <select
                        className="lp-input"
                        value={stepAssessmentId}
                        onChange={(event) => {
                          setStepAssessmentId(event.target.value);
                        }}
                        required
                      >
                        <option value="" disabled>
                          Choose a published assessment
                        </option>
                        {assessments.map((assessment) => (
                          <option key={assessment.id} value={assessment.id}>
                            {assessment.title} (pass at {assessment.passingScore}%)
                          </option>
                        ))}
                      </select>
                    )}
                    <p className="text-sm text-[var(--lp-ink-muted)]">
                      The step completes when the employee&apos;s latest attempt passes.
                    </p>
                  </div>
                ) : null}
                {stepType === "quiz" ? (
                  <div className="space-y-4 rounded-[var(--lp-radius)] border border-[var(--lp-border)] p-4 md:col-span-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-semibold">Quiz questions</h3>
                      <button
                        type="button"
                        className="lp-btn lp-btn--secondary"
                        onClick={() => {
                          setQuizQuestions((current) => [...current, newQuizQuestion()]);
                        }}
                      >
                        Add question
                      </button>
                    </div>
                    {quizQuestions.length === 0 ? (
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        Add at least one question. Mark the correct option for each.
                      </p>
                    ) : null}
                    {quizQuestions.map((question, questionIndex) => (
                      <div
                        key={questionIndex}
                        className="space-y-2 rounded-[var(--lp-radius)] border border-[var(--lp-border)] p-3"
                      >
                        <div className="flex gap-2">
                          <input
                            className="lp-input"
                            placeholder={`Question ${String(questionIndex + 1)}`}
                            value={question.text}
                            onChange={(event) => {
                              updateQuizQuestion(questionIndex, { text: event.target.value });
                            }}
                            required
                          />
                          <button
                            type="button"
                            className="lp-btn lp-btn--secondary"
                            onClick={() => {
                              setQuizQuestions((current) =>
                                current.filter((_, i) => i !== questionIndex),
                              );
                            }}
                          >
                            Remove
                          </button>
                        </div>
                        {question.options.map((option, optionIndex) => (
                          <div key={optionIndex} className="flex items-center gap-2">
                            <input
                              type="radio"
                              name={`correct-${String(questionIndex)}`}
                              title="Correct answer"
                              checked={question.correctOption === optionIndex}
                              onChange={() => {
                                updateQuizQuestion(questionIndex, {
                                  correctOption: optionIndex,
                                });
                              }}
                            />
                            <input
                              className="lp-input"
                              placeholder={`Option ${String(optionIndex + 1)}`}
                              value={option}
                              onChange={(event) => {
                                updateQuizQuestion(questionIndex, {
                                  options: question.options.map((current, i) =>
                                    i === optionIndex ? event.target.value : current,
                                  ),
                                });
                              }}
                              required
                            />
                            <button
                              type="button"
                              className="lp-btn lp-btn--secondary"
                              disabled={question.options.length <= 2}
                              onClick={() => {
                                const options = question.options.filter(
                                  (_, i) => i !== optionIndex,
                                );
                                updateQuizQuestion(questionIndex, {
                                  options,
                                  correctOption: Math.min(
                                    question.correctOption,
                                    options.length - 1,
                                  ),
                                });
                              }}
                            >
                              ×
                            </button>
                          </div>
                        ))}
                        <button
                          type="button"
                          className="lp-btn lp-btn--secondary"
                          onClick={() => {
                            updateQuizQuestion(questionIndex, {
                              options: [...question.options, ""],
                            });
                          }}
                        >
                          Add option
                        </button>
                      </div>
                    ))}
                  </div>
                ) : null}
                <button
                  type="submit"
                  disabled={pending}
                  className="lp-btn lp-btn--primary"
                >
                  Add step
                </button>
              </form>
              <button
                type="button"
                disabled={pending || steps.length === 0}
                onClick={onPublish}
                className="lp-btn lp-btn--secondary mt-4"
              >
                Publish journey
              </button>
            </Surface>
          </Reveal>
        ) : null}

        {journey?.status === "published" ? (
          <Reveal delay={2}>
            <Surface>
              <h2 className="text-lg font-semibold">Assign to department</h2>
              <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                Every active employee in the department gets this journey. Anyone
                who already has it is skipped, so it is safe to re-run.
              </p>
              <form onSubmit={onAssignDepartment} className="mt-4 grid gap-3 md:grid-cols-2">
                <select className="lp-input" name="departmentId" required defaultValue="">
                  <option value="" disabled>
                    Choose a department
                  </option>
                  {departments.map((department) => (
                    <option key={department.id} value={department.id}>
                      {department.name}
                    </option>
                  ))}
                </select>
                <input className="lp-input" name="startsAt" type="date" />
                <button
                  type="submit"
                  disabled={pending}
                  className="lp-btn lp-btn--primary md:col-span-2"
                >
                  Assign journey
                </button>
              </form>
            </Surface>
          </Reveal>
        ) : null}

        <Reveal delay={3}>
          <Surface className="overflow-hidden p-0">
            <div className="flex items-start justify-between gap-4 border-b border-[var(--lp-border)] px-5 py-4">
              <div>
                <h2 className="text-lg font-semibold">Versions</h2>
                <p className="text-sm text-[var(--lp-ink-muted)]">
                  Publishing a draft locks it; start a new version to keep editing.
                </p>
              </div>
              <div className="flex gap-2">
                {journey?.status === "published" ? (
                  <button
                    type="button"
                    disabled={pending}
                    onClick={onCreateVersion}
                    className="lp-btn lp-btn--primary"
                  >
                    New version
                  </button>
                ) : null}
                <button
                  type="button"
                  disabled={pending}
                  onClick={onClone}
                  className="lp-btn lp-btn--secondary"
                >
                  Clone
                </button>
              </div>
            </div>
            {versions.length === 0 ? (
              <div className="p-5">
                <EmptyState dense title="No versions yet" description="Version history appears after the first publish." />
              </div>
            ) : (
              <ol className="divide-y divide-[var(--lp-border)]">
                {versions.map((version) => (
                  <li key={version.version} className="flex items-center justify-between gap-4 px-5 py-4">
                    <div>
                      <p className="font-medium">Version {version.version}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {version.status} · {version.stepCount} steps
                      </p>
                    </div>
                    {journey?.status === "published" &&
                    version.version < journey.currentVersion ? (
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          onRollback(version.version);
                        }}
                        className="lp-btn lp-btn--secondary"
                      >
                        Roll back to this
                      </button>
                    ) : null}
                  </li>
                ))}
              </ol>
            )}
          </Surface>
        </Reveal>

        <Reveal delay={3}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Steps</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{steps.length} steps</p>
            </div>
            {steps.length === 0 ? (
              <div className="p-5">
                <EmptyState dense title="No steps yet" description="Add at least one step before publishing." />
              </div>
            ) : (
              <ol className="divide-y divide-[var(--lp-border)]">
                {steps.map((step) => (
                  <li key={step.id} className="flex items-start justify-between gap-4 px-5 py-4">
                    <div>
                      <p className="font-medium">
                        {step.position}. {step.title}
                      </p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {step.stepType}
                        {step.stepType === "quiz"
                          ? ` · ${String(
                              Array.isArray(step.config?.questions) ? step.config.questions.length : 0,
                            )} questions`
                          : ""}
                        {step.instructions ? ` · ${step.instructions}` : ""}
                      </p>
                    </div>
                    {journey?.status === "draft" ? (
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          onDeleteStep(step.id);
                        }}
                        className="lp-btn lp-btn--secondary"
                      >
                        Delete
                      </button>
                    ) : null}
                  </li>
                ))}
              </ol>
            )}
          </Surface>
        </Reveal>
      </div>
      );
}
