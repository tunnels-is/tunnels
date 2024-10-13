import { useParams, useNavigate } from 'react-router-dom'
import GLOBAL_STATE from "../../state";

import React, { useEffect, useState } from 'react';
import { githubDark, githubLight } from '@uiw/codemirror-theme-github';
import { json } from "@codemirror/lang-json";
import { vim } from "@replit/codemirror-vim";
import CodeMirror from '@uiw/react-codemirror';

const FullJSON = (props) => {
	const state = GLOBAL_STATE()
	const [useVim, setUseVim] = useState(true)
	const [original, setOriginal] = useState({})
	const [editorRerender, setEditorRerender] = useState(0)

	const reset = () => {
		setOriginal(props.data)
		setError("")
		setEditorRerender(editorRerender + 1)
		if (props.reset) {
			props.reset()
		}
	}

	const close = () => {
		props.close()
		state.globalRerender()
	}

	useEffect(() => {
		setOriginal(props.data)
	}, [])

	let ext = [json()]
	if (useVim) {
		ext.push(vim())
	}

	return (
		<div className="editor-popup editor-inline">
			<div className="editor-topbar">
				{!props.readOnly &&
					<div className="vim" onClick={() => setUseVim(!useVim)}>VIMode</div>
				}
				{props.save &&
					<div className="save" onClick={() => props.save()}>Save</div>
				}
				{!props.readOnly &&
					<div className="reset" onClick={() => reset()}>Reset</div>
				}
				<div className="close" onClick={() => close()}>Close</div>
				{props.delete &&
					<div className="delete" onClick={() => props.delete()}>Delete</div>
				}
			</div>
			{(props.error !== "") &&
				<div className="error">{props.error}</div>
			}
			<CodeMirror
				key={editorRerender}
				value={JSON.stringify(original, null, 4)}
				onChange={(newValue) => props.onChange(newValue)}
				theme={state.darkMode ? githubDark : githubLight}
				extensions={ext}
				readOnly={props.readOnly}
				basicSetup={{ autocompletion: true }}
			/>
		</div>
	);

}

export default FullJSON;
