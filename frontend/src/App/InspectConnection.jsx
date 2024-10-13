import { useParams, useNavigate } from 'react-router-dom'
import GLOBAL_STATE from "../state";

import React, { useState } from 'react';
import { githubDark, githubLight } from '@uiw/codemirror-theme-github';
import { json } from "@codemirror/lang-json";
import CodeMirror from '@uiw/react-codemirror';

const InspectConnection = () => {
	const state = GLOBAL_STATE()
	const [error, setError] = useState(false)
	const [editorRerender, setEditorRerender] = useState(0)
	const { id } = useParams();
	const navigate = useNavigate();


	let config = undefined
	if (state.modifiedConfig) {
		config = state.modifiedConfig
	} else if (state.Config) {
		config = state.Config
	}

	let connection = undefined
	config?.Connections?.forEach(c => {
		if (c.WindowsGUID === id) {
			connection = c
		}
	})

	if (!connection) {
		return (
			<div></div>
		)
	}

	const Reset = () => {
		console.log("RESET!")
		state.RemoveModifiedConfig()
		state.rerender()
		setEditorRerender(editorRerender + 1)
		setError(false)
	}

	const Delete = (id) => {
		state.DeleteConnection(id)
		navigate("/connections")

	}

	const UpdateConnection = (data) => {
		let x = undefined
		try {
			x = JSON.parse(data)
			setError(false)
		} catch (e) {
			setError(true)
			console.dir(e)
			return
		}

		let modCons = state.GetModifiedConnections()
		let found = false
		modCons.forEach((n, i) => {
			if (n.WindowsGUID === x.WindowsGUID) {
				modCons[i] = x
				found = true
			}
		})

		if (!found) {
			modCons.push(x)
		}

		state.SaveConnectionsToModifiedConfig(modCons)
		state.rerender()
	}


	return (
		<>

			<CodeMirror
				key={editorRerender}
				value={JSON.stringify(connection, null, 4)}
				onChange={(newValue) => UpdateConnection(newValue)}
				theme={state.getDarkMode() ? githubDark : githubLight}
				extensions={[json()]}
				basicSetup={{ autocompletion: true }}
			/>
		</>
	);

}

export default InspectConnection;
