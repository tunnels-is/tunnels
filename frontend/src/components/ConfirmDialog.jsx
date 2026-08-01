import { useStore } from "@/store/store"

const ConfirmDialog = () => {
	const confirm = useStore((s) => s.confirm)
	const close = useStore((s) => s.closeConfirm)
	if (!confirm) return null

	const destructive = /delete|disconnect|logout|remove/i.test(confirm.title + confirm.subtitle)

	return (
		<dialog className="modal modal-open" onClose={close}>
			<div className="modal-box max-w-sm">
				<h3 className="text-lg font-bold">{confirm.title}</h3>
				<p className="py-3 text-sm opacity-70">{confirm.subtitle}</p>
				<div className="modal-action">
					<button className="btn btn-ghost btn-sm" onClick={close}>
						Cancel
					</button>
					<button
						className={"btn btn-sm " + (destructive ? "btn-error" : "btn-primary")}
						onClick={() => {
							confirm.onConfirm?.()
							close()
						}}
					>
						Confirm
					</button>
				</div>
			</div>
			<div className="modal-backdrop" onClick={close} />
		</dialog>
	)
}

export default ConfirmDialog
