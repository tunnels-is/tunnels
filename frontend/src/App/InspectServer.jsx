import React, { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import FormKeyValue from "./component/formkeyvalue";
import GLOBAL_STATE from "../state"
import KeyValue from "./component/keyvalue";

const InspectServer = () => {
	const { id } = useParams()
	const [tag, setTag] = useState("")
	const [serverID, setServerID] = useState(id)
	const [server, setServer] = useState()
	const state = GLOBAL_STATE("inspect-server")
	const navigate = useNavigate()

	let org = state?.Org

	const Create = async () => {
		let user = state.User
		if (!user) {
			return
		}
		let server = {}
		server.Tag = tag
		server.OrgID = user.OrgID
		server.Admin = user._id

		let resp = await state.API_CreateServer(server)
		if (resp?.status === 200) {
			setServer(resp.data)
			setServerID(resp.data._id)
			state.renderPage("inspect-server")
			navigate("/inspect/server/" + resp.data._id)
		}
	}

	const Save = async (id) => {
		let resp = await state.API_UpdateServer(id)
		if (resp?.status === 200) {
			state.renderPage("inspect-server")
		}
	}

	useEffect(() => {
		let srv = undefined
		state?.ModifiedServers?.forEach(g => {
			if (g._id === serverID) {
				srv = g
				return
			}
		})
		if (!srv) {
			state?.PrivateServers?.forEach(g => {
				if (g._id === serverID) {
					srv = g
					return
				}
			})
		}
		if (!server && srv) {
			setServer(srv)
		}
	}, [])

	if (!serverID) {
		return (
			<div className="ab group-wrapper">
				<div className="title">Create Server</div>
				<FormKeyValue label={"Tag"} value={<input onChange={(e) => setTag(e.target.value)} type="text" />} />
				<div className="button" onClick={() => Create()}>Create</div>
			</div>
		)
	}

	if (!server) {
		return (
			<div className="ab group-wrapper">
				<div className="title">Server Not Found: {serverID}</div>
			</div>
		)
	}


	return (
		<div className="ab group-wrapper">
			<div className="title">{server.Tag}</div>

			<FormKeyValue label={"Tag"} value={
				<input type="text" value={server.Tag} onChange={(e) => {
					state.UpdateModifiedServer(server, "Tag", e.target.value)
				}} />
			} />

			<FormKeyValue label={"Serial"} value={
				<input type="text" value={server.Serial} onChange={(e) => {
					state.UpdateModifiedServer(server, "Serial", e.target.value)
				}} />
			} />

			<KeyValue label={"ID"} value={server._id} />
			<KeyValue label={"Org"} value={org?.Name} />
			<div className="button" onClick={() => Save(server._id)}>Update</div>


		</div>
	)
}

export default InspectServer;
