import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const navigate = useNavigate()
	const [pserver, setPServer] = useState(undefined)
	const [nserver, setNServer] = useState(undefined)

	useEffect(() => {
		state.GetPrivateServers()
	}, [])

	const UpdateServer = async () => {
		console.log("mewo")
		console.dir(pserver)
		let resp = await state.API_UpdateServer(pserver)
		if (resp?.status === 200) {
			state.renderPage("pservers")
		}
	}
	const serverUpdateOpts = {
		baseClass: "private-server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root_OrgID: true,
			root__id: true,
		},
		saveButton: UpdateServer,
	}

	const CreateServer = async () => {
		let user = state.User
		if (!user) {
			return
		}
		nserver.OrgID = user.OrgID
		nserver.Admin = user._id

		let resp = await state.API_CreateServer(nserver)
		if (resp?.status === 200) {
			state.renderPage("pservers")
		}
	}
	const serverCreateOpts = {
		baseClass: "private-server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root_OrgID: true,
			root__id: true,
		},
		saveButton: CreateServer,
	}


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
						// navigate("/inspect/server/" + server._id)
						setPServer(server)
						state.renderPage("pservers")
					}
				},
				{ type: "text", minWidth: "300px", value: server._id },
				{ type: "text", minWidth: "350px", value: server.Serial },
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
		{ value: "ID", minWidth: "300px" },
		{ value: "Cert Serial", minWidth: "350px" },
		{ value: "Tunnel" },
	]

	if (nserver !== undefined) {
		return (
			<div className="connections">
				<div className="back" onClick={() => setNServer(undefined)}>Back to server</div>
				<ObjectEditor
					opts={serverCreateOpts}
					object={nserver}
				/>
			</div>
		)
	}

	if (pserver !== undefined) {
		return (
			<div className="connections">
				<div className="back" onClick={() => setPServer(undefined)}>Back to server</div>
				<ObjectEditor
					opts={serverUpdateOpts}
					object={pserver}
				/>
			</div>
		)
	}


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
					text: "New Server",
					click: function() {
						let user = state.User
						if (!user) {
							return
						}

						setNServer({
							OrgID: user.OrgID,
							Admin: user._id,
							Tag: "",
							Serial: "",
						})
					}
				}}

			/>
		</div >
	);

}

export default PrivateServers;
