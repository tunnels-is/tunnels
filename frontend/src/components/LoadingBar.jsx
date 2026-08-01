import { useStore } from "@/store/store"

const LoadingBar = () => {
	const loading = useStore((s) => s.loading)
	if (!loading) return null
	return (
		<div className="fixed left-0 top-0 z-[90] w-full">
			<progress className="progress progress-primary h-1 w-full" />
			{loading.msg && (
				<div className="mx-auto mt-1 w-fit rounded-b-md bg-base-100 px-3 py-0.5 text-xs opacity-70 shadow">
					{loading.msg}
				</div>
			)}
		</div>
	)
}

export default LoadingBar
