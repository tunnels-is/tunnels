import { useEffect } from "react"
import React from 'react';
import EDITOR from "../editor"
import GLOBAL_STATE from "../state";


const ObjectEditor = () => {
	const state = GLOBAL_STATE("oe")

	EDITOR.original = {
		key: "meow1",
		x: 'kjslfd',
		arr: ["meow2"],
		xx: true,
		obj: {
			arr2: ["meow3", "meow4"]
		},
		obj2: {
			xx: [1, 2, 3, 4, 5]
		}
	}

	useEffect(() => {
		// EDITOR.createForm(EDITOR.original)
	}, [])

	console.dir(EDITOR)
	// let data = EDITOR.original
	let xxx = { ...state?.Config }

	const save = () => {
		console.log("save")
		console.dir(xxx)
	}

	const transformType = (t) => {
		if (t === "boolean") {
			return "checkbox"
		} else if (t === "string") {
			return "text"
		}
		return t
	}


	const walk = (data) => {
		if (!data) { return }
		let dom = []
		switch (typeof data) {
			case "object":
				dom = walkObj(data, "", data)
				break
			case "array":
				dom = walkArr(data, "", data)
				break
			default:
				<div>no data..</div>
		}
		return React.createElement("div", { key: "root", className: "object-editor" },
			dom
		)
	}

	const walkObj = (data, id, obj) => {
		let title = data["Title"]
		if (title === undefined) {
			title = data["Tag"]
		}
		if (title === undefined) {
			title = data["Name"]
		}

		console.log('TITLE:', title)
		return React.createElement("div", { key: id, className: "obj-grp" },
			title !== undefined ? React.createElement("div", { key: id + "_title", className: "title" }, title) : null,
			...Object.keys(data).map(v => {
				return switchIT(data[v], v, data)
			})
		)
	}

	const walkArr = (data, id, obj) => {
		return React.createElement("div", { key: id, className: "arr-grp" },
			id !== "" ? React.createElement("div", { key: id + "_title", className: "title" }, id) : null,
			...data.map((v, i) => {
				return switchIT(v, i, data)
			})
		)
	}

	const switchIT = (data, id, obj) => {
		// console.log("TYPE:", data, "==", Object.prototype.toString.call(data))
		switch (Object.prototype.toString.call(data)) {
			case "[object Array]":
				return walkArr(data, id, obj)
			case "[object Object]":
				return walkObj(data, id, obj)
			default:
				let ot = typeof data
				let input = undefined
				if (ot === "boolean") {
					input = React.createElement("input",
						{
							key: id,
							className: "",
							checked: Boolean(data),
							type: transformType(ot),
							onClick: (e) => {
								obj[id] = e.target.checked ? true : false
								console.log("check:", obj[id])
								state.renderPage("oe")
							},
						})
				} else {
					input = React.createElement("input",
						{
							key: id,
							className: "",
							value: data,
							type: transformType(ot),
							onChange: (e) => {
								console.log("on change!")
								if (ot === "number") {
									obj[id] = Number(e.target.value)
								} else {
									obj[id] = String(e.target.value)
								}
								state.renderPage("oe")
							},
						})

				}

				return React.createElement("div", { key: id + "wrap", className: "input-wrap" },
					React.createElement("div", { key: id + "wrap", className: "label" }, id),
					input)
		}
	}

	console.log("config")
	console.dir(state.Config)
	return (
		<>
			{walk(xxx)}
		</>
	)
}

export default ObjectEditor;
