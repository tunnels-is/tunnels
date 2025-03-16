import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import { BarLoader } from "react-spinners";
import WS from "../ws";
import STORE from "../store";


const ScreenLoader = () => {
	const state = GLOBAL_STATE("loader")
	const [filter, setFilter] = useState("");
	const [fullscreen, setFullscreen] = useState(false)
	const [hide, setHide] = useState(false)


	const reloadSocket = () => {
		WS.sockets["logs"] = undefined
		WS.NewSocket(WS.GetURL("logs"), "logs", WS.ReceiveLogEvent)
	}


	let logs = state.logs
	let classes = "bottom-loader"
	let hideClasses = "item"
	let hideLabel = "hide"
	let fullLabel = "expand"
	if (fullscreen) {
		fullLabel = "minimize"
		classes += " bottom-loader-fullscreen"
	}
	if (hide) {
		hideClasses += " show"
		hideLabel = "show"
		classes += " bottom-loader-hide"
	} else {
		hideClasses += " hide"
	}

	return (
		<div className={classes}  >
			<div className="control-bar">
				<div className="item fullscreen"
					onClick={() => reloadSocket()}
				>
					reload
				</div>
				<div className="item fullscreen"
					onClick={() => setFullscreen(!fullscreen)}
				>
					{fullLabel}
				</div>

				<div className={hideClasses}
					onClick={() => setHide(!hide)}
				>
					{hideLabel}
				</div>

				<div className="item clear"
					onClick={() => {
						STORE.Cache.DelObject("logs")
						state.logs = []
						state.renderPage("loader")
					}}
				>
					clear
				</div>
			</div>

			{state.loading?.msg &&
				<>
					<div key={state.loading?.tag} className="title">
						{state.loading?.msg}
					</div>
					<BarLoader width="100%" className="bar-loader" />
				</>
			}

			<div className="logs-window custom-scrollbar">
				{logs?.toReversed().map((line, index) => {
					let splitLine = line.split(" || ")
					let error = line.includes("| ERROR |")
					let debug = line.includes("| DEBUG |")
					let info = line.includes("| INFO  |")

					if (filter !== "") {
						if (!line.includes(filter)) {
							return
						}
					}
					return (
						<div className={`line`} key={index}>

							<div className="time">{splitLine[0]}</div>

							{info &&
								<div className="info">{splitLine[1]}</div>
							}
							{error &&
								<div className="error">{splitLine[1]}</div>
							}
							{debug &&
								<div className="debug">{splitLine[1]}</div>
							}
							{!debug && !error && !info &&
								<div className="text"> {splitLine[1]}</div>
							}

							<div className="func">{splitLine[2]}</div>
							<div className="text"> {splitLine.splice(3, 20).join("||")}</div>
						</div >
					)
				})}
			</div>
		</div>
	)
}

export default ScreenLoader;
