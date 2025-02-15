import { useEffect, useState } from "react"
import React from 'react';
import GLOBAL_STATE from "../state";


const ObjectEditor = (props) => {
	const state = GLOBAL_STATE("oe")
	const [trigger, setTrigger] = useState({ id: 1 })

	const reload = () => {
		let xx = { ...trigger }
		xx.id += 1
		setTrigger(xx)
	}

	const noItems = React.createElement("div",
		{
			key: "no-items",
			className: "no-items",
		},
		"no items"
	)



	const transformType = (t) => {
		if (t === "boolean") {
			return "checkbox"
		} else if (t === "string") {
			return "text"
		}
		return t
	}


	const makeX = (data, type, ns, id, parent, opts) => {
		if (data === undefined || data === null) {
			return { type: type, id: id, ns: ns, parent: parent, origin: data }
		}
		let d = String(opts.meta.depth)
		let x = {
			id: id,
			ns: String(ns),
			parent: parent,
			type: type,
			key: ns + id + d + opts.meta.index,
			ds: "d" + d,
			className: "d" + d + " " + id + " " + ns,
			extraClasses: "",
			delButton: undefined,
			newButton: undefined,
			nested: [],
			origin: data,
			title: ""
		}

		if (opts.meta.index !== undefined) {
			x.index = String(opts.meta.index)
			x.className = x.className + "_" + x.index
		}

		if (x.type === "array") {
			if (!containsObj(data)) {
				x.className = x.className + " arr_grp_column"
			}
		}
		if (type === "object") {
			x.className += " obj_grp"
		} else if (type === "array") {
			x.className += " arr_grp"
		}

		if (x.title === "") {
			if (x.type === "array") {
				x.title = x.id
			} else if (x.type !== "object") {
				x.title = x.id
			}
		}

		if (props.opts.titles && props.opts.titles[x.ns] !== undefined) {
			if (x.index !== undefined && x.index !== "") {
				x.title = props.opts.titles[x.ns + "_" + x.index]
			} else {
				x.title = props.opts.titles[x.ns]
			}
		}

		if (x.title === undefined) {
			x.title = data["Tag"]
		}
		if (x.title === undefined) {
			x.title = data["Name"]
		}
		if (x.title === undefined) {
			x.title = data["Title"]
		}


		if (opts.delButtons !== undefined) {
			x.delButton = opts.delButtons[ns]
		}
		if (x.index === undefined) {
			if (opts.newButtons !== undefined) {
				x.newButton = opts.newButtons[ns]
			}
		}

		if (parent.type === "array" && x.delButton === undefined) {
			x.delButton = () => {
				parent.origin.splice(id, 1)
				reload()
			}
		}
		return x
	}

	const getType = (data) => {
		switch (Object.prototype.toString.call(data)) {
			case "[object Array]":
				return "array"
			case "[object Object]":
				return "object"
			default:
				let to = typeof data
				switch (to) {
					case "boolean":
						return "boolean"
					case "number":
						return "number"
					default:
						return "string"
				}
		}
	}

	const containsObj = (data) => {
		let hasObjects = false
		switch (Object.prototype.toString.call(data)) {
			case "[object Object]":
				Object.keys(data).forEach(v => {
					switch (Object.prototype.toString.call(data[v])) {
						case "[object Array]":
							hasObjects = true
							return
						case "[object Object]":
							hasObjects = true
							return
					}
				})
				break
			case "[object Array]":
				data.forEach((v) => {
					switch (Object.prototype.toString.call(v)) {
						case "[object Array]":
							hasObjects = true
							return
						case "[object Object]":
							hasObjects = true
							return
					}
				})
				break
		}
		return hasObjects
	}


	const switchX = (data, id, ns, parent, opts) => {
		switch (getType(data)) {
			case "array":
				walkA(data, id, ns, parent, opts)
				break
			case "object":
				walkO(data, id, ns, parent, opts)
				break
			default:
				let to = typeof data
				ns = ns + "_" + id
				let x = makeX(data, to, ns, id, parent, opts)
				parent.nested.push(x)
		}
		return
	}

	const walkO = (data, id, ns, parent, opts) => {
		opts.meta.depth += 1
		if (parent.type === "array") {
			ns = (ns ? ns : "")
		} else {
			ns = (ns ? ns + "_" : "") + id
		}
		let x = makeX(data, "object", ns, id, parent, opts)
		parent.nested.push(x)

		let objKeys = []
		let otherKeys = []
		let boolKeys = []
		Object.keys(data).forEach(v => {
			if (typeof data[v] === "object") {
				objKeys.push(v)
			} else if (typeof data[v] === "boolean") {
				boolKeys.push(v)
			} else {
				otherKeys.push(v)
			}
		})

		otherKeys.forEach(k => {
			opts.meta.index = undefined
			switchX(data[k], k, ns, x, opts)
		})

		boolKeys.forEach(k => {
			opts.meta.index = undefined
			switchX(data[k], k, ns, x, opts)
		})

		objKeys.forEach(k => {
			opts.meta.index = undefined
			switchX(data[k], k, ns, x, opts)
		})

		opts.meta.depth -= 1
	}

	const walkA = (data, id, ns, parent, opts) => {
		opts.meta.depth += 1
		ns = (ns ? ns + "_" : "") + id
		let x = makeX(data, "array", ns, id, parent, opts)
		parent.nested.push(x)

		opts.meta.index = undefined
		data.forEach((v, i) => {
			opts.meta.index = i
			switchX(v, i, ns, x, opts)
		})
		opts.meta.index = undefined

		opts.meta.depth -= 1
	}

	const makeMeta = (data, opts) => {
		let bt = getType(data)
		opts.meta = {
			index: undefined,
			parent: {},
			depth: 0,
			rootKeys: [],
			rootArrays: [],
			rootObjects: [],
			nested: [],
			type: bt,
			origin: data,
			id: "root",
			ns: "root",
		}


		// console.log("basetype", bt)
		if (bt === "object") {
			Object.keys(data).forEach(k => {
				if ((data[k] === null || data[k] === undefined) && props.opts.defaults) {
					let def = props.opts.defaults["root_" + k]
					if (def) {
						data[k] = def
					}
				}
				let gt = getType(data[k])

				// TODO .. ignore undefined, null and functions
				if (gt !== "object" && gt !== "array") {
					let x = makeX(data[k], gt, "root_" + k, k, opts.meta, opts)
					opts.meta.rootKeys.push(x)
				} else {
					if (containsObj(data[k]) === false) {
						let x = makeX(data[k], gt, "root_" + k, k, opts.meta, opts)
						if (gt === "array") {
							opts.meta.index = undefined
							data[k].forEach((v, i) => {
								opts.meta.index = i
								switchX(v, i, "root", x, opts)
							})
							opts.meta.index = undefined
							opts.meta.rootArrays.push(x)
						} else if (gt === "object") {
							Object.keys(data[k]).forEach(v => {
								opts.meta.index = undefined
								switchX(data[k][v], v, "root", x, opts)
							})
							opts.meta.rootObjects.push(x)
						}
					} else {
						if (gt === "array") {
							// console.log("KEY:", k)
							opts.meta.index = undefined
							walkA(data[k], k, "root", opts.meta, opts)
						} else if (gt === "object") {
							opts.meta.index = undefined
							walkO(data[k], k, "root", opts.meta, opts)
						}
					}
				}
			})

			// do main walk
			// 		walkO(data, "root", data, "", root, opts)
		} else if (bt === "array") {
			data.forEach((v, i) => {
				let gt = getType(v)
				if (gt !== "object" && gt !== "array") {
					let x = makeX(v, gt, "root", i, opts.meta, opts)
					opts.meta.rootKeys.push(x)
				} else {
					if (containsObj(v) === false) {
						let x = makeX(v, gt, "root", i, opts.meta, opts)
						if (gt === "array") {
							opts.meta.index = undefined
							v.forEach((vv, i) => {
								opts.meta.index = i
								switchX(vv, i, "root", x, opts)
							})
							opts.meta.index = undefined
							opts.meta.rootArrays.push(x)
						} else if (gt === "object") {
							Object.keys(v).forEach(vv => {
								opts.meta.index = undefined
								switchX(v[vv], vv, "root", x, opts)
							})
							opts.meta.rootObjects.push(x)
						}
					} else {
						if (gt === "array") {
							opts.meta.index = undefined
							walkA(v, i, "root", opts.meta, opts)
						} else if (gt === "object") {
							opts.meta.index = undefined
							walkO(v, i, "root", opts.meta, opts)
						}
					}
				}
			})
		} else {
			// ????
		}


		// console.log("META")
		// console.dir(opts.meta)
	}

	const makeInput = (x) => {
		if (props.opts.hidden && props.opts.hidden["root_" + x.id] === true) {
			return
		}
		let input = null
		let label = null
		if (x.type === "boolean") {
			input = React.createElement("input",
				{
					key: x.key + "_checkbox",
					className: "checkbox",
					checked: x.parent.origin[x.id],
					type: transformType(x.type),
					onClick: (e) => {
						x.parent.origin[x.id] = e.target.checked ? true : false
						// console.log("check:", x.parent.origin[x.id])
						state.renderPage("oe")
					},
				})
		} else {
			let disabled = false
			if (props.opts.disabled && props.opts.disabled["root_" + x.id] === true) {
				disabled = true
			} else if (props.opts.readOnly === true) {
				disabled = true
			}
			input = React.createElement("input",
				{
					key: x.key + "_input",
					className: "input",
					value: x.parent.origin[x.id],
					type: transformType(x.type),
					// disabled: props.opts.disabled ? props.opts.disabled["root_" + x.id] : false,
					disabled: disabled,
					onChange: (e) => {
						// console.log("change")
						// console.dir(x.id)
						// console.dir(x.parent.origin)
						// console.dir(x.parent.origin[x.id])
						if (x.type === "number") {
							x.parent.origin[x.id] = Number(e.target.value)
						} else {
							x.parent.origin[x.id] = String(e.target.value)
						}
						state.renderPage("oe")
					},
				})
		}

		if (x.parent.type === "array") {
			label = React.createElement("div", {
				key: x.key + "_rem_button",
				className: "rem_button",
				onClick: () => {
					x.parent.origin.splice(x.id, 1)
					state.renderPage("oe")
				}
			}, "X")

		} else {
			label = React.createElement("div", {
				key: x.key + "_label",
				className: "label",
			}, x.title)
		}


		return React.createElement("div", {
			key: x.key + "_input_wrap",
			className: x.id + " input_wrap " + (x.type !== "boolean" ? "bottom_border" : "") + " " + x.ns,
		},
			label,
			input,
		)
	}

	const walkNested = (x) => {

		let sub = []
		if (x.type === "object" || x.type === "array") {
			if (x.nested?.length < 1) {
				sub.push(noItems)
			} else {
				x.nested?.map(xx => {
					sub.push(walkNested(xx))
				})
			}

		} else {
			return makeInput(x)
		}

		if (props.opts.hidden && props.opts.hidden["root_" + x.id] === true) {
			return
		}

		let titleD = null
		let newB = null
		let delB = null
		let topB = null

		titleD = x.title !== undefined ? React.createElement("div", {
			key: x.key + "_title",
			className: "title",
		}, x.title) : null

		if (titleD === null && x.type === "array") {
			titleD = x.id !== "" ? React.createElement("div", {
				key: x.key + "_title",
				className: "title",
			},
				x.id) : null

		}

		newB = x.newButton !== undefined ? React.createElement("div", {
			key: x.key + "_new_button",
			className: "new_button",
			onClick: () => {
				x.newButton(x.origin)
				reload()
			}
		}, "Add") : null

		delB = x.delButton !== undefined ? React.createElement("div", {
			key: x.key + "_del_button",
			className: "del_button",
			onClick: () => {
				x.delButton(x.parent.origin)
				reload()
			}
		}, x.type === "object" ? "delete" : "x") : null

		if (delB !== null || titleD !== null || newB !== null) {
			topB = React.createElement("div", {
				key: x.key + "_top_bar",
				className: "top_bar",
			},
				titleD,
				newB,
				delB,
			)

		}

		return React.createElement("div",
			{
				key: x.key,
				className: x.className + " " + x.extraClasses,
			},
			topB,
			...sub,
		)
	}

	const makeDom = (opts) => {
		let rootKeys = []
		opts.meta.rootKeys.map((k, i) => {
			if (k.type !== "boolean") {
				rootKeys.push(makeInput(k))
			}
		})
		opts.meta.rootKeys.map((k, i) => {
			if (k.type === "boolean") {
				rootKeys.push(makeInput(k))
			}
		})

		let rootArrays = []
		opts.meta.rootArrays.map((k, i) => {
			rootArrays.push(walkNested(k))
		})

		let rootObjects = []
		opts.meta.rootObjects.map((k, i) => {
			rootObjects.push(walkNested(k))
		})

		let rootKeyDom = React.createElement("div", {
			key: "root_keys",
			id: "root_keys",
			className: "root_keys obj_grp",
		},
			...rootKeys,
			...rootArrays,
			...rootObjects
		)

		let nested = []
		opts.meta.nested.map(n => {
			nested.push(walkNested(n))
		})

		return React.createElement("div", {
			key: "root" + opts.baseClass,
			className: "object-editor " + opts.baseClass
		},
			rootKeyDom,
			...nested,
		)

	}


	makeMeta(props.object, props.opts)
	// console.log(props.opts.meta)
	// console.log("config")
	// console.dir(state.Config)
	return (<>
		<div className="object-editor-save" onClick={() => props.opts.saveButton()}>Save</div>
		{makeDom(props.opts)}
	</>)
}

export default ObjectEditor;
