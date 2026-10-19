import { forwardRef, useId } from 'react';

export const inputClass =
  'w-full h-[30px] px-2.5 rounded-md border border-line bg-bg text-fg placeholder:text-muted focus:outline-none focus:border-accent focus:ring-2 focus:ring-accent/25 disabled:opacity-60 disabled:cursor-not-allowed';

export const Input = forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(function Input(
  { className = '', ...rest },
  ref,
) {
  return <input ref={ref} className={`${inputClass} ${className}`} {...rest} />;
});

export const Select = forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(function Select(
  { className = '', ...rest },
  ref,
) {
  return <select ref={ref} className={`${inputClass} pr-7 ${className}`} {...rest} />;
});

type FieldProps = {
  label: string;
  hint?: string;
  error?: string | null;
  className?: string;
  /** Renders the control; spread the given props onto it to wire label, hint and error. */
  children: (props: { id: string; 'aria-describedby'?: string; 'aria-invalid'?: boolean }) => React.ReactNode;
};

/** Label + control + hint/error, with ids wired for assistive tech. */
export function Field({ label, hint, error, className = '', children }: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined;
  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      <label htmlFor={id} className="text-[11.5px] font-semibold text-muted">
        {label}
      </label>
      {children({ id, 'aria-describedby': describedBy, 'aria-invalid': error ? true : undefined })}
      {hint && (
        <p id={hintId} className="text-[11.5px] text-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-[11.5px] text-danger">
          {error}
        </p>
      )}
    </div>
  );
}
