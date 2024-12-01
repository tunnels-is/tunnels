import React, { useEffect } from "react";
import GLOBAL_STATE from "../state";
import KeyValue from "./component/keyvalue";
import CustomTable from "./component/table";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";


const Org = () => {
	const state = GLOBAL_STATE("org")
	const navigate = useNavigate()

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

	return (
		<div className="ab org-wrapper">
			<div className="info frame-spacing panel">
				<div className="title">{state?.Org?.Name}</div>
				<KeyValue label={"Address"} value={state?.Org?.Address} />
				<KeyValue label={"Email"} value={state?.Org?.Email} />
				<KeyValue label={"Phone"} value={state?.Org?.Phone} />
				<KeyValue label={"Domains"} value={state?.Org?.Domains?.join(", ")} />
				<KeyValue label={"Information"} value={state?.Org?.Informatgion} />
				<KeyValue label={"ID"} value={state?.Org?._id} />
			</div>


			<NewTable
				tableID={"org-groups"}
				title={"Groups"}
				className="group-table"
				header={headers}
				rows={rows}
				placeholder={"Search.."}
				button={{
					text: "Create",
					click: function(e) {
						navigate("/inspect/group")
					}
				}}
			/>

		</div>
	)
}

export default Org;
