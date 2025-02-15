import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import KeyValue from "./component/keyvalue";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";


const Org = () => {
	const state = GLOBAL_STATE("org")
	const navigate = useNavigate()
	const [editOrg, setEditOrg] = useState(undefined)

	const updateOpts = {
		baseClass: "org-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root__id: true,

		},
		titles: {
			root_Information: "Additional Info",
			root_MangerID: "Manager ID"
		},
		hidden: {
			root_Groups: true,
		},
		newButtons: {
			root_Domains: (obj) => {
				obj.push("new-domain.local")
			},
		},
		saveButton: async () => {
			console.log('Update ORG')
			console.dir(editOrg)
			await state.API_UpdateOrg(editOrg)
		}
	}
	const createOpts = {
		baseClass: "org-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		titles: {
			root_Information: "Additional Info",
			root_MangerID: "Manager ID"
		},
		hidden: {
			root_Groups: true,
		},
		newButtons: {
			root_Domains: (obj) => {
				obj.push("new-domain.local")
			},
		},
		saveButton: async () => {
			console.log('SAVE ORG')
			console.dir(dummyOrg)
			await state.API_CreateOrg(dummyOrg)
		}
	}

	const dummyOrg = {
		Name: "",
		Address: "",
		Domains: ["myorg.local"],
		ManagerID: "",
		Information: "",
		Email: "",
		Phone: "",
	}

	useEffect(() => {
		let x = async () => {
			await state.GetBackendState()
			await state.API_GetOrg()
		}

		x()
	}, [])

	const generateGroupTable = (org) => {
		let rows = []

		org?.Groups?.forEach(g => {
			let row = {}
			row.items = [
				{
					type: "text", value: g.Tag, color: "blue", click: function(e) {
						navigate("/inspect/group/" + g._id)
					}
				},
				{ type: "text", value: g._id },
				{ type: "text", align: "right", value: g.Users ? Object.keys(g.Users).length : 0 },
				{ type: "text", value: g.Servers ? Object.keys(g.Servers).length : 0 },
			]
			rows.push(row)
		})

		return rows
	}


	let rows = generateGroupTable(state?.Org)
	const headers = [
		{ value: "Tag" },
		{ value: "ID" },
		{ value: "Users", align: "right" },
		{ value: "Servers" }
	]

	if (editOrg) {
		return (
			<ObjectEditor
				opts={updateOpts}
				object={editOrg}
			/>
		)
	}

	return (
		<div className="ab org-wrapper">
			{!state.Org &&
				<>
					<ObjectEditor
						opts={createOpts}
						object={dummyOrg}
					/>
				</>
			}
			{state.Org &&
				<>
					<div className="info frame-spacing panel">
						<div className="title"
							onClick={() => {
								setEditOrg(state.Org)
							}}
						>{state?.Org?.Name}</div>
						<KeyValue label={"Address"} value={state?.Org?.Address} />
						<KeyValue label={"Email"} value={state?.Org?.Email} />
						<KeyValue label={"Phone"} value={state?.Org?.Phone} />
						<KeyValue label={"Domains"} value={state?.Org?.Domains?.join(", ")} />
						<KeyValue label={"Information"} value={state?.Org?.Information} />
						<KeyValue label={"ID"} value={state?.Org?._id} />
					</div>

					<NewTable
						background={true}
						tableID={"org-groups"}
						title={"Groups"}
						className="group-table"
						header={headers}
						rows={rows}
						placeholder={"Search.."}
						button={{
							text: "Create",
							click: function() {
								navigate("/inspect/group")
							}
						}}
					/>
				</>
			}

		</div>
	)
}

export default Org;
