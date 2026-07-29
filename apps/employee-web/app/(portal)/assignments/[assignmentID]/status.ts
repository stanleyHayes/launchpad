/** Humanizes a step/assignment status code for display. */
export function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}
