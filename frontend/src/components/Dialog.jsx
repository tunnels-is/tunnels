const Dialog = ({ open, onClose, title, children, actions, wide }) => {
	if (!open) return null
	return (
		<dialog className="modal modal-open">
			<div className={"modal-box " + (wide ? "max-w-3xl" : "max-w-md")}>
				<button className="btn btn-circle btn-ghost btn-sm absolute right-2 top-2" onClick={onClose}>
					✕
				</button>
				{title && <h3 className="mb-4 text-lg font-bold">{title}</h3>}
				{children}
				{actions && <div className="modal-action">{actions}</div>}
			</div>
			<div className="modal-backdrop" onClick={onClose} />
		</dialog>
	)
}

export default Dialog
