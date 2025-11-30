import React from "react";
import { useAtomValue } from "jotai";
import { loadingAtom } from "../stores/uiStore";
import { BarLoader } from "react-spinners";

const ScreenLoader = () => {
	const loading = useAtomValue(loadingAtom);

	if (!loading?.msg) {
		return (<></>)
	}

	return (
		<div className={"new-loader"}  >
			<div key={loading?.tag} className="l-title">
				{loading?.msg}
			</div>
			<BarLoader width="100%" color={"blue"} height={"8px"} className="bar-loader" />
		</div>
	)
}

export default ScreenLoader;
