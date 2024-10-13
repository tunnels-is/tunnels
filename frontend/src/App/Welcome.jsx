import React from "react";
import STORE from "../store"
import * as runtime from "../../wailsjs/runtime/runtime"
import GLOBAL_STATE from "../state";

const Welcome = () => {
	const state = GLOBAL_STATE()

	const Copy = (value) => {
		window.open(value, "_blank")
		// try {
		//   state.ConfirmAndExecute("", "clipboardCopy", 10000, value, "Copy link to clipboard ?", () => {
		//     if (navigator?.clipboard) {
		//       navigator.clipboard.writeText(value);
		//     }
		//     runtime.ClipboardSetText(value)
		//   })
		// } catch (e) {
		//   alert(e)
		//   console.log(e)
		// }
	}

	return (
		<div className="support-wrapper">

			<div className="support-table">
				{STORE.SupportPlatforms.map(s => {
					return (
						<div className="row" key={s.name}>
							<div className="name"
								onClick={() => Copy(s.link)}
							>{s.name}</div>
							{s.type === "email" &&
								<div className="link"><a href={`mailto: ${s.link}`} >{s.link}</a></div>
							}
							{s.type === "link" &&
								<div className="link"><a href={s.link} target="_blank" >{s.link}</a></div>
							}
						</div>
					)
				})}

			</div>
		</div >
	)
}

export default Welcome;
