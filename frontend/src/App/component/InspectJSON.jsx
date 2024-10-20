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
	const [useVim, setUseVim] = useState(true)
	const [original, setOriginal] = useState({})
	const [editorRerender, setEditorRerender] = useState(0)
	// console.log("INSPECT")
	//

	const reset = () => {
		setOriginal(state.editorData)
		state.editorError = ""
		setEditorRerender(editorRerender + 1)
	}

	const close = () => {
		setOriginal(undefined)
		state.resetEditor()
		state.globalRerender()
	}

	useEffect(() => {
		setOriginal(state?.editorData)
	}, [])

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
				{!state.editorReadOnly &&
					<div className="reset" onClick={() => reset()}>Reset</div>
				}
				<div className="close" onClick={() => close()}>Close</div>
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
				key={editorRerender}
				value={JSON.stringify(original, null, 4)}
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
