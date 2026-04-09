import { clsx } from "clsx";
import type { HTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";
import { ChevronDown, type LucideIcon } from "lucide-react";

type Tone = "neutral" | "primary" | "success" | "warning" | "danger";

const toneClassNames: Record<Tone, string> = {
  neutral: "border-border bg-surface text-text-secondary",
  primary: "border-accent/20 bg-accent-muted text-accent",
  success: "border-online/20 bg-online/10 text-online",
  warning: "border-warning/20 bg-warning/10 text-warning",
  danger: "border-offline/20 bg-offline/10 text-offline",
};

export const selectFieldClassName =
  "ui-select w-full appearance-none rounded-2xl border border-border bg-surface px-4 py-3 pr-11 text-sm text-text-primary outline-none hover:border-border-hover disabled:cursor-not-allowed disabled:opacity-60";

export function PageHeader({
  title,
  actions,
  className,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <header className={clsx("flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between", className)}>
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-text-primary md:text-3xl">{title}</h1>
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-3">{actions}</div> : null}
    </header>
  );
}

export function SectionCard({
  title,
  actions,
  children,
  className,
  contentClassName,
}: {
  title?: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
}) {
  return (
    <section className={clsx("rounded-3xl border border-border bg-card shadow-panel", className)}>
      {title || actions ? (
        <div className="flex flex-col gap-4 border-b border-border px-5 py-5 md:flex-row md:items-start md:justify-between md:px-6">
          {title ? <h2 className="text-base font-semibold text-text-primary">{title}</h2> : <div />}
          {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
        </div>
      ) : null}
      <div className={clsx("px-5 py-5 md:px-6", contentClassName)}>{children}</div>
    </section>
  );
}

export function MetricCard({
  icon: Icon,
  label,
  value,
  tone = "neutral",
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  meta?: string;
  tone?: Tone;
}) {
  const iconTone = {
    neutral: "bg-surface-subtle text-text-secondary",
    primary: "bg-accent-muted text-accent",
    success: "bg-online/10 text-online",
    warning: "bg-warning/10 text-warning",
    danger: "bg-offline/10 text-offline",
  }[tone];

  return (
    <article className="rounded-3xl border border-border bg-card p-5 shadow-panel">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2">
          <p className="text-sm font-medium text-text-secondary">{label}</p>
          <p className="text-2xl font-semibold tracking-tight text-text-primary">{value}</p>
        </div>
        <span className={clsx("flex h-11 w-11 items-center justify-center rounded-2xl", iconTone)}>
          <Icon className="h-5 w-5" />
        </span>
      </div>
    </article>
  );
}

export function StatusBadge({
  tone = "neutral",
  label,
  className,
}: {
  tone?: Tone;
  label: string;
  className?: string;
}) {
  return (
    <span
      className={clsx(
        "inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium",
        toneClassNames[tone],
        className,
      )}
    >
      <span className="h-2 w-2 rounded-full bg-current" aria-hidden="true" />
      {label}
    </span>
  );
}

export function Banner({
  tone = "neutral",
  role,
  children,
  className,
}: {
  tone?: Tone;
  role?: "alert" | "status";
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      role={role}
      className={clsx(
        "rounded-2xl border px-4 py-3 text-sm leading-6",
        toneClassNames[tone],
        className,
      )}
    >
      {children}
    </div>
  );
}

export function SelectField({
  className,
  children,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <div className="relative">
      <select {...props} className={clsx(selectFieldClassName, className)}>
        {children}
      </select>
      <span className="pointer-events-none absolute inset-y-0 right-4 flex items-center text-text-muted">
        <ChevronDown className="h-4 w-4" />
      </span>
    </div>
  );
}

export function EmptyState({
  icon: Icon,
  title,
  action,
  className,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={clsx("flex flex-col items-center justify-center rounded-3xl border border-dashed border-border bg-surface px-6 py-14 text-center", className)}>
      <span className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-surface-subtle text-text-secondary">
        <Icon className="h-6 w-6" />
      </span>
      <h3 className="text-lg font-semibold text-text-primary">{title}</h3>
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}

export function SurfaceBox({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return <div className={clsx("rounded-2xl border border-border bg-surface p-4", className)} {...props} />;
}
