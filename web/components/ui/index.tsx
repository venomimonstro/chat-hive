import type { ButtonHTMLAttributes, HTMLAttributes, InputHTMLAttributes, ReactNode } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  fullWidth?: boolean;
};

export function Button({ variant = 'primary', fullWidth = false, className = '', ...props }: ButtonProps) {
  const classes = ['uiButton', `uiButton--${variant}`, fullWidth ? 'uiButton--full' : '', className]
    .filter(Boolean)
    .join(' ');
  return <button className={classes} {...props} />;
}

export function IconButton({ className = '', ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button className={`uiIconButton ${className}`.trim()} {...props} />;
}

export function Avatar({ name, size = 'md' }: { name: string; size?: 'sm' | 'md' | 'lg' }) {
  return (
    <span className={`uiAvatar uiAvatar--${size}`} aria-hidden="true">
      {name.trim().slice(0, 1).toUpperCase() || '?'}
    </span>
  );
}

export function Badge({ children, tone = 'accent' }: { children: ReactNode; tone?: 'accent' | 'neutral' | 'danger' }) {
  return <span className={`uiBadge uiBadge--${tone}`}>{children}</span>;
}

export function SearchField({ className = '', ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className={`uiSearch ${className}`.trim()}>
      <span aria-hidden="true">⌕</span>
      <input type="search" {...props} />
    </label>
  );
}

export function Surface({ className = '', ...props }: HTMLAttributes<HTMLElement>) {
  return <section className={`uiSurface ${className}`.trim()} {...props} />;
}

export function Skeleton({ width = '100%', height = 16 }: { width?: string | number; height?: number }) {
  return <span className="uiSkeleton" aria-hidden="true" style={{ width, height }} />;
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return (
    <div className="uiEmptyState">
      <div className="uiEmptyState__mark" aria-hidden="true">✦</div>
      <strong>{title}</strong>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function Sheet({ title, children, open = false }: { title: string; children: ReactNode; open?: boolean }) {
  if (!open) return null;
  return (
    <div className="uiSheetBackdrop" role="presentation">
      <section className="uiSheet" role="dialog" aria-modal="true" aria-label={title}>
        <div className="uiSheet__grab" aria-hidden="true" />
        <header><strong>{title}</strong></header>
        {children}
      </section>
    </div>
  );
}
