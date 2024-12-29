import { useParams, useNavigate } from 'react-router-dom'
import GLOBAL_STATE from "../../state";

import React, { useEffect, useState } from 'react';
import { githubDark, githubLight } from '@uiw/codemirror-theme-github';
import { json } from "@codemirror/lang-json";
import { vim } from "@replit/codemirror-vim";
import CodeMirror from '@uiw/react-codemirror';

const InspectJSON = () => {
	const state = GLOBAL_STATE()
	const [error, setError] = useState(false)
	const [useVim, setUseVim] = useState(false)
	const [rerend, setRerend] = useState(undefined)

	const reset = () => {
		state.editorData = JSON.parse(JSON.stringify(state.editorOriginal))
		state.editorError = ""
		// state.editorRerender = state.editorRerender + 1
		setRerend(rerend + 1)
		state.globalRerender()
	}

	const close = () => {
		// setOriginal(undefined)
		state.editorOriginal = undefined
		state.editorData = undefined
		state.resetEditor()
		state.globalRerender()
	}

	useEffect(() => {
		if (state.editorOriginal === undefined) {
			console.log("reset! useeff")
			state.editorOriginal = JSON.parse(JSON.stringify(state.editorData))
		}
		console.dir(state.editorOriginal)
		console.dir(state.editorData)
		// setOriginal(state?.editorData)
	}, [rerend])

	let ext = [json()]
	if (useVim) {
		ext.push(vim())
	}


	return (
		<div className="editor-popup">
			<div className="editor-topbar">
				{!state.editorReadOnly &&
					<div className="vim" onClick={() => setUseVim(!useVim)}>VIMode</div>
				}
				{state.editorSave &&
					<div className="save" onClick={() => state.editorSave()}>Save</div>
				}
				<div className="close" onClick={() => close()}>Close</div>
				{!state.editorReadOnly &&
					<div className="reset" onClick={() => reset()}>Reset</div>
				}



				{state.editorExtraButtons.map((b) => {
					return (
						<div className="extra" onClick={() => {
							b.func()
						}}>{b.title}</div>
					)
				})}

				{state.editorDelete &&
					<div className="delete" onClick={() => {
						state.editorDelete()
						close()
					}}>Delete</div>
				}

			</div>
			{state?.editorError &&
				<div className="error">{state.editorError}</div>
			}
			<CodeMirror
				key={rerend}
				value={JSON.stringify(state.editorData, null, 4)}
				onChange={(newValue) => state?.editorOnChange(newValue)}
				theme={state.getDarkMode() ? githubDark : githubLight}
				extensions={ext}
				readOnly={state.editorReadOnly}
				basicSetup={{ autocompletion: true }}
			/>
		</div>
	);

}

export default InspectJSON;
