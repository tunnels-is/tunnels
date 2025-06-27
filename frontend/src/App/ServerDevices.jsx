import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import GenericTable from "./GenericTable";
import { TableCell } from "@/components/ui/table";
import { useParams } from "react-router-dom";

const ServerDevices = () => {
	const state = GLOBAL_STATE("devices")
	const [connectedDevices, setConnectedDevices] = useState([])
	const { id } = useParams()

	const getConnectedDevices = async () => {
		let server = undefined
		state.PrivateServers.forEach((s, i) => {
			if (s._id === id) {
				server = state.PrivateServers[i]
			}
		})
		if (!server) {
			return
		}
		let resp = await state.callController("https://" + server.IP, null, "POST", "/v3/devices", {}, false, false)
		if (resp.status === 200) {
			setConnectedDevices(resp.data)
			state.renderPage("devices")
		}
	}

	useEffect(() => {
		getConnectedDevices()
	}, [])

	let table = {
		data: connectedDevices.Devices,
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		columns: {
			Created: true,
		},
		customColumns: {
			IP: (obj) => {
				return <TableCell className={""}>
					{obj.DHCP?.IP.join(".")}
				</TableCell >
			},
			Hostname: (obj) => {
				return <TableCell className={""}>
					{obj.DHCP?.Hostname}
				</TableCell >
			},
			Ports: (obj) => {
				return <TableCell className={""}>
					{obj.StartPort} - {obj.EndPort}
				</TableCell >
			},

		},
		columnFormat: {
			Created: (obj) => {
				return dayjs(obj.Created).fromNow()
			}
		},
		Btn: {
			Delete: (obj) => {
				deleteDevice(obj._id)
			},
		},
		columnClass: {},
		headerFormat: {
			Created: () => {
				return "Connected"
			}
		},
		headers: ["Created", "IP", "Hostname", "Ports"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
	}

	return (
		<div className="">
			<div className="text-lg text-white">Connected: {connectedDevices.DHCPAssigned}</div>
			<GenericTable table={table} />
		</div >
	)
}

export default ServerDevices;
