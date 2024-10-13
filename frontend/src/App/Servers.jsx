import React, { useEffect } from "react";
import GLOBAL_STATE from "../state";
import CustomTable from "./component/table";
import CustomSelect from "./component/CustomSelect";
import NewTable from "./component/newtable";

const Servers = () => {
	const state = GLOBAL_STATE("servers")

	const popEditor = (r) => {
		state.editorData = r
		state.editorReadOnly = true
		state.globalRerender()
	}

	useEffect(() => {
		state.GetServers()
	}, [])

	const generateServerTable = () => {
		let rows = []


		state.Servers?.forEach(server => {

			let con = undefined
			let conButton = function() {
				state.ConfirmAndExecute(
					"success",
					"connect",
					10000,
					"",
					"Connect to " + server.Tag,
					() => {
						state.connectToVPN(undefined, server)
					})
			}

			state.State.ActiveConnections?.forEach((x) => {
				if (x.ServerID === server._id) {
					con = x
					return
				}
			})

			if (con) {
				conButton = function() {
					state.ConfirmAndExecute(
						"success",
						"disconnect",
						10000,
						"",
						"Disconnect from " + server.Tag,
						() => {
							state.disconnectFromVPN(con)
						})
				}

			}

			let country = "icon"
			if (server.Country !== "") {
				country = server.Country.toLowerCase()
			}

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
					type: "img",
					align: "right",
					className: "flag",
					value: "https://raw.githubusercontent.com/tunnels-is/media/master/nl-website/v2/flags/" + country + ".svg"
				},
				{
					type: "text",
					value: server.Tag,
					color: "blue",
					click: function() {
						popEditor(server)
					}
				},
				{ type: "text", value: server.IP },
				{
					type: "select",
					opts: opts,
					value: <CustomSelect
						parentkey={server._id}
						className={"clickable"}
						placeholder={"Assign"}
						setValue={(opt) => {
							state.changeServerOnConnection(opt.value, server._id)
						}}
						options={opts}
					></CustomSelect>,
				},
				{
					type: "text",
					value: con ? "Disconnect" : "Connect",
					color: con ? "red" : "green",
					click: conButton,
				},
			]
			rows.push(row)
		})

		return rows
	}


	let rows = generateServerTable()
	const headers = [
		{ value: "" },
		{ value: "Tag" },
		{ value: "IP" },
		{ value: "Connection" },
		{ value: "" }
	]

	return (
		<div className="ab router-wrapper">

			<NewTable
				tableID={"public-servers"}
				className="router-table"
				placeholder={"Search for a server.."}
				header={headers}
				rows={rows}
			/>
		</div >
	);

}

export default Servers;
