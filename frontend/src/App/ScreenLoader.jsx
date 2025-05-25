import React from "react";
import GLOBAL_STATE from "../state";
import { BarLoader } from "react-spinners";

const ScreenLoader = () => {
	const state = GLOBAL_STATE("loader")
	if (!state.loading?.msg) {
		return (<></>)
	}

	return (
		<div className={"new-loader"}  >
			<div key={state.loading?.tag} className="l-title">
				{state.loading?.msg}
			</div>
			<BarLoader width="100%" color={"blue"} height={"8px"} className="bar-loader" />
		</div>
	)
}

export default ScreenLoader;
