import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost";

const styles: Record<Variant, string> = {
  primary: "lp-btn lp-btn--primary",
  secondary: "lp-btn lp-btn--secondary",
  ghost: "lp-btn lp-btn--ghost",
};

/**
 * Buttons are styled by the active design system (`.lp-btn*` in styles.css):
 * raised dual-shadow surfaces in neumorphic, frosted in glass, puffy in clay.
 */
export function Button({
  variant = "primary",
  className = "",
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  children: ReactNode;
}) {
  return (
    <button className={`${styles[variant]}${className ? ` ${className}` : ""}`} {...props}>
      {children}
    </button>
  );
}
