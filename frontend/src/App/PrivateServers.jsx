import React, { useEffect } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const navigate = useNavigate()

	useEffect(() => {
		state.GetPrivateServers()
	}, [])

	const generateServerTable = () => {
		let rows = []
		state.PrivateServers?.forEach(server => {


			let opts = []
			state?.Config?.Connections?.map(c => {
				if (c.ServerID === server._id) {
					opts.push({ value: c.Tag, key: c.Tag, selected: true })
				} else {
					opts.push({ value: c.Tag, key: c.Tag, selected: false })
				}
			})


			let row = {}
			row.items = [
				{
					type: "text",
					value: server.Tag,
					color: "blue",
					click: function() {
						navigate("/inspect/server/" + server._id)
					}
				},
				{ type: "text", value: server._id },
				{ type: "text", value: server.Serial },
				{
					type: "select",
					opts: opts,
					value: <CustomSelect
						parentkey={server._id}
						className={""}
						placeholder={"Assign"}
						setValue={(opt) => {
							state.changeServerOnConnection(opt.value, server._id)
						}}
						options={opts}
					></CustomSelect>,
				}
			]
			rows.push(row)
		})

		return rows
	}


	let rows = generateServerTable()
	const headers = [
		{ value: "Tag" },
		{ value: "ID" },
		{ value: "Cert Serial" },
		{ value: "Tunnel" },
	]

	return (
		<div className="ab private-server-wrapper">

			<NewTable
				title={"Private VPN Servers"}
				tableID={"private-servers"}
				className="router-table"
				placeholder={"Search .."}
				header={headers}
				background={true}
				rows={rows}
				button={{
					text: "Create",
					click: function() {
						navigate("/inspect/server")
					}
				}}

			/>
		</div >
	);

}

export default PrivateServers;
