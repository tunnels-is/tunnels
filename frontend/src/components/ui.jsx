export const Page = ({ actions, children }) => (
	<div className="min-h-screen bg-base-200 pl-14">
		<div className="w-full p-6">
			{actions && <div className="mb-4 flex flex-wrap items-center justify-end gap-2">{actions}</div>}
			{children}
		</div>
	</div>
)

export const Card = ({ title, description, actions, children, className = "" }) => (
	<div className={"card border border-base-300 bg-base-100 " + className}>
		<div className="card-body gap-0 p-5">
			{(title || actions || description) && (
				<div className="mb-3">
					{(title || actions) && (
						<div className="flex items-start justify-between gap-3">
							{title && <h2 className="min-w-0 flex-1 text-sm font-semibold tracking-tight">{title}</h2>}
							{actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
						</div>
					)}
					{description && (
						<p className="mt-0.5 w-full text-[11px] leading-relaxed opacity-50">{description}</p>
					)}
				</div>
			)}
			{children}
		</div>
	</div>
)

export const Field = ({ label, hint, children }) => (
	<fieldset className="fieldset">
		<legend className="fieldset-legend text-xs">{label}</legend>
		{children}
		{hint && <p className="label text-xs">{hint}</p>}
	</fieldset>
)

export const TextField = ({ label, hint, ...props }) => (
	<Field label={label} hint={hint}>
		<input className="input input-sm w-full" {...props} />
	</Field>
)

export const Toggle = ({ label, checked, onChange, disabled, warning, hint }) => (
	<label className={"label cursor-pointer justify-start gap-3 py-1 " + (disabled ? "opacity-60" : "")}>
		<input
			type="checkbox"
			className="toggle toggle-primary toggle-sm shrink-0"
			checked={!!checked}
			disabled={disabled}
			onChange={onChange}
		/>
		<span className="min-w-0 flex-1">
			<span className="label-text block text-sm leading-snug">{label}</span>
			{warning && (
				<span className="mt-0.5 block text-[11px] font-medium leading-snug text-warning">{warning}</span>
			)}
			{hint && !warning && (
				<span className="mt-0.5 block text-[11px] leading-snug opacity-50">{hint}</span>
			)}
		</span>
	</label>
)

export const InfoRow = ({ label, value, mono }) => (
	<div className="flex items-center justify-between gap-4 border-b border-base-200 py-1.5 text-sm last:border-0">
		<span className="shrink-0 opacity-60">{label}</span>
		<span className={"truncate text-right " + (mono ? "font-mono text-xs" : "")}>{value}</span>
	</div>
)

export const Toolbar = ({ children }) => (
	<div className="mb-4 flex flex-wrap items-center gap-3">{children}</div>
)
