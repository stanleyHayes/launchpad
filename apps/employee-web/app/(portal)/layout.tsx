import { EmployeeShell } from "@/components/employee-shell";

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return <EmployeeShell>{children}</EmployeeShell>;
}
