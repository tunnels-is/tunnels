import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import GenericTable from "./GenericTable";
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
		let s = { ...state.User?.ControlServer }
		s.Host = server.IP
		let resp = await state.callController(s, "POST", "/v3/devices", {}, false, false)
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
			Activity: true,
			IP: true,
			Token: true,
			Hostname: true,
			Ports: true,
			CPU: true,
			RAM: true,
			Disk: true,
		},
		customColumns: {
		},
		columnFormat: {
			Created: (obj) => {
				return dayjs(obj.Created).format("HH:mm:ss DD-MM-YYYY")
			},
			Activity: (obj) => {
				return obj.DHCP?.Activity ? dayjs(obj.DHCP.Activity).format("HH:mm:ss DD-MM-YYYY") : ""
			},
			IP: (obj) => {
				return obj.DHCP?.IP ? obj.DHCP.IP.join(".") : ""
			},
			Token: (obj) => {
				return obj.DHCP?.Token ? obj.DHCP.Token : ""
			},
			Hostname: (obj) => {
				return obj.DHCP?.Hostname ? obj.DHCP.Hostname : ""
			},
			Ports: (obj) => {
				return "" + obj.StartPort + " - " + obj.EndPort
			},
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
		headers: ["Created", "Activity", "IP", "Device", "Ports", "CPU", "RAM", "DISK", "IQ", "EQ"],
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
