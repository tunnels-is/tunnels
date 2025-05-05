import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import NewTable from "./component/newtable";

const Devices = () => {
	const [devices, setDevices] = useState([])
	const state = GLOBAL_STATE("groups")

	const getDevices = async () => {
		let resp = await state.callController(null, null, "POST", "/v3/device/list", { Offset: 0, Limit: 1000 }, false, false)
		if (resp.status === 200) {
			setDevices(resp.data)
		}
	}

	useEffect(() => {
		getDevices()
	}, [])


	const generateDeviceTable = (devices) => {
		let rows = []
		devices.forEach((u, i) => {
			let row = {}
			row.items = [
				{
					type: "text",
					color: "blue",
					value: u.Email,
				},
				{
					type: "text",
					minWidth: "250px",
					value: u._id,
				},
				{
					type: "text",
					value: dayjs(u.Added).format("DD-MM-YYYY HH:mm:ss"),
				},
				{
					type: "text",
					color: "red",
					click: () => {
						// removeEntity(id, u._id, "user")
					},
					value: "Delete"
				},
			]
			rows.push(row)
		});

		return rows
	}


	let deviceRow = generateDeviceTable(devices)
	const devicesHeaders = [
		{ value: "Email" },
		{ value: "ID", minWidth: "250px" },
		{ value: "Added" },
		{ value: "" }
	]

	return (
		<div className="ab device-wrapper">
			<NewTable
				background={true}
				title={""}
				className="device-table"
				header={devicesHeaders}
				rows={deviceRow}
				placeholder={"Search.."}
				button={{
					text: "Add Device",
					// click: () => setDialog(true)
				}}
			/>
		</div >
	)
}

export default Devices;
