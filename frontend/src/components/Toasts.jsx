import { useStore } from "@/store/store"

const Toasts = () => {
	const toasts = useStore((s) => s.toasts)
	if (toasts.length === 0) return null
	return (
		<div className="toast toast-top toast-end z-[100]">
			{toasts.map((t) => (
				<div key={t.id} className={"alert py-2 text-sm whitespace-pre-line " + (t.type === "error" ? "alert-error" : "alert-success")}>
					<span>{t.msg}</span>
				</div>
			))}
		</div>
	)
}

export default Toasts
