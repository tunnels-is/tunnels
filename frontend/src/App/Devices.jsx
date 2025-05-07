import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import NewTable from "./component/newtable";
import { v4 as uuidv4 } from "uuid";
import ObjectEditorDialog from "./component/ObjectEditorDialog";
import { Edit } from "lucide-react";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";

const Devices = () => {
	const [devices, setDevices] = useState([])
	const [device, setDevice] = useState(undefined)
	const state = GLOBAL_STATE("groups")
	const [editModalOpen, setEditModalOpen] = useState(false)

	const getDevices = async () => {
		let resp = await state.callController(null, null, "POST", "/v3/device/list", { Offset: 0, Limit: 1000 }, false, false)
		if (resp.status === 200) {
			setDevices(resp.data)
			state.renderPage("groups")
		}
	}
	const deleteDevice = async (id) => {
		let ok = await state.callController(null, null, "POST", "/v3/device/delete", { DID: id }, false, true)
		if (ok === true) {
			let d = devices.filter((d) => d._id !== id)
			setDevices([...d])
			state.renderPage("groups")
		}
	}

	const saveDevice = async () => {
		let resp = undefined
		if (device._id !== undefined) {
			resp = await state.callController(null, null, "POST", "/v3/device/update", { Device: device }, false, false)
			if (resp.status === 200) {
				state.renderPage("groups")
			}
		} else {
			resp = await state.callController(null, null, "POST", "/v3/device/create", { Device: device }, false, false)
			if (resp.status === 200) {
				setDevice(resp.data)
				state.renderPage("groups")
			}
		}

	}

	const deviceCreateOpts = {
		baseClass: "",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root__id: true,
			root_CreatedAt: true,
		},
		saveButton: saveDevice,
	}

	const newDevice = () => {
		setDevice({ Tag: "", Groups: [] })
		setEditModalOpen(true)
	}

	useEffect(() => {
		getDevices()
	}, [])


	const generateDeviceTable = (devices) => {
		let rows = []
		devices.forEach((d, i) => {
			let row = {}
			row.items = [
				{
					type: "text",
					color: "blue",
					value: d.Tag,
				},
				{
					type: "text",
					minWidth: "250px",
					value: d._id,
				},
				{
					type: "text",
					value: dayjs(d.Added).format("DD-MM-YYYY HH:mm:ss"),
				},
				{
					type: "text",
					value: <Edit className="h-4 w-4 mr-1" />,
					click: () => {
						setDevice(d)
						setEditModalOpen(true)

					}
				},
				{
					type: "text",
					color: "red",
					click: () => {
						deleteDevice(d._id)
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
		{ value: "Tag" },
		{ value: "ID", minWidth: "250px" },
		{ value: "Added" },
		{ value: "" },
		{ value: "" }
	]

	return (
		<div className="">
			<NewTable
				background={true}
				title={""}
				className="device-table"
				header={devicesHeaders}
				rows={deviceRow}
				placeholder={"Search.."}
				button={{
					text: "Add Device",
					click: () => {
						newDevice()
					}
				}}
			/>

			<NewObjectEditorDialog
				open={editModalOpen}
				onOpenChange={setEditModalOpen}
				object={device}
				title="Device"
				description=""
				readOnly={false}
				saveButton={() => {
					console.log("save")
					saveDevice()
				}}
				onChange={(key, value, type) => {
					device[key] = value
					console.log(key, value, type)
				}}
			/>

		</div >
	)
}

export default Devices;
