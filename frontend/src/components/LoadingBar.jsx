import { useEffect, useRef, useState } from "react"
import { useStore } from "@/store/store"


const SHOW_DELAY_MS = 180

const MIN_VISIBLE_MS = 400

const EXIT_MS = 280

const LoadingBar = () => {
	const loading = useStore((s) => s.loading)
	const [visible, setVisible] = useState(false)
	const [msg, setMsg] = useState("")
	const [leaving, setLeaving] = useState(false)

	const shownAtRef = useRef(0)
	const showTimerRef = useRef(null)
	const hideTimerRef = useRef(null)
	const leaveTimerRef = useRef(null)

	useEffect(() => {
		const clearHide = () => {
			clearTimeout(hideTimerRef.current)
			clearTimeout(leaveTimerRef.current)
			hideTimerRef.current = null
			leaveTimerRef.current = null
		}

		if (loading) {
			clearHide()
			setLeaving(false)
			setMsg(loading.msg || "")

			if (visible) return

			clearTimeout(showTimerRef.current)
			showTimerRef.current = setTimeout(() => {
				shownAtRef.current = Date.now()
				setVisible(true)
				setLeaving(false)
			}, SHOW_DELAY_MS)

			return () => {
				clearTimeout(showTimerRef.current)
				showTimerRef.current = null
			}
		}

		clearTimeout(showTimerRef.current)
		showTimerRef.current = null

		if (!visible) return

		const elapsed = Date.now() - shownAtRef.current
		const remain = Math.max(0, MIN_VISIBLE_MS - elapsed)

		clearHide()
		hideTimerRef.current = setTimeout(() => {
			setLeaving(true)
			leaveTimerRef.current = setTimeout(() => {
				setVisible(false)
				setLeaving(false)
				setMsg("")
			}, EXIT_MS)
		}, remain)

		return clearHide

	}, [loading])

	useEffect(
		() => () => {
			clearTimeout(showTimerRef.current)
			clearTimeout(hideTimerRef.current)
			clearTimeout(leaveTimerRef.current)
		},
		[],
	)

	if (!visible) return null

	return (
		<div
			className="pointer-events-none fixed inset-0 z-[90] flex items-center justify-center"
			role="status"
			aria-live="polite"
			aria-busy="true"
		>
			{}
			<div
				className="absolute inset-0 bg-base-200/45 backdrop-blur-[3px]"
				style={{
					opacity: leaving ? 0 : 1,
					transition: `opacity ${EXIT_MS}ms ease`,
				}}
			/>

			{}
			<div
				className={
					"loading-panel relative flex min-w-[12rem] max-w-[min(90vw,22rem)] flex-col items-center gap-5 " +
					"rounded-2xl border border-base-300/80 bg-base-100/95 px-9 py-8 " +
					"shadow-2xl shadow-black/15 ring-1 ring-black/5 backdrop-blur-xl " +
					"dark:shadow-black/50 dark:ring-white/5"
				}
				style={{
					opacity: leaving ? 0 : 1,
					transform: leaving ? "scale(0.94) translateY(8px)" : "scale(1) translateY(0)",
					transition: `opacity ${EXIT_MS}ms ease, transform ${EXIT_MS}ms cubic-bezier(0.22, 1, 0.36, 1)`,
				}}
			>
				{}
				<div className="relative h-14 w-14">
					{}
					<div className="absolute inset-0 rounded-full border-2 border-primary/10" />

					{}
					<div className="loading-spin absolute inset-0 rounded-full border-[2.5px] border-transparent border-t-primary border-r-primary/30" />

					{}
					<div className="loading-spin-rev absolute inset-[5px] rounded-full border border-dashed border-primary/35" />

					{}
					<div className="loading-spin absolute inset-0">
						<span className="absolute left-1/2 top-0 h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary shadow-[0_0_12px_var(--color-primary)]" />
					</div>

					{}
					<div className="loading-pulse absolute inset-[18px] rounded-full bg-primary" />
				</div>

				<p className="max-w-full truncate px-1 text-center text-[13px] font-medium tracking-tight text-base-content/75">
					{msg || "Loading…"}
				</p>
			</div>

			<style>{`
				@keyframes loading-spin {
					to { transform: rotate(360deg); }
				}
				@keyframes loading-spin-rev {
					to { transform: rotate(-360deg); }
				}
				@keyframes loading-pulse {
					0%, 100% { transform: scale(0.8); opacity: 0.45; }
					50%      { transform: scale(1.2); opacity: 1; }
				}
				@keyframes loading-panel-in {
					from { opacity: 0; transform: scale(0.9) translateY(12px); }
					to   { opacity: 1; transform: scale(1) translateY(0); }
				}
				.loading-panel {
					animation: loading-panel-in 0.35s cubic-bezier(0.22, 1, 0.36, 1) both;
				}
				.loading-spin {
					animation: loading-spin 0.9s cubic-bezier(0.45, 0.05, 0.55, 0.95) infinite;
				}
				.loading-spin-rev {
					animation: loading-spin-rev 2.6s linear infinite;
				}
				.loading-pulse {
					animation: loading-pulse 1.35s ease-in-out infinite;
				}
			`}</style>
		</div>
	)
}

export default LoadingBar
