import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import GenericTable from "./GenericTable";

const Devices = () => {
	const [devices, setDevices] = useState([])
	const state = GLOBAL_STATE("devices")
	const [device, setDevice] = useState(undefined)
	const [editModalOpen, setEditModalOpen] = useState(false)

	const getDevices = async (offset, limit) => {
		let resp = await state.callController(null, null, "POST", "/v3/device/list", { Offset: offset, Limit: limit }, false, false)
		if (resp.status === 200) {
			setDevices(resp.data)
			state.renderPage("devices")
		}
	}
	const deleteDevice = async (id) => {
		let ok = await state.callController(null, null, "POST", "/v3/device/delete", { DID: id }, false, true)
		if (ok === true) {
			let d = devices.filter((d) => d._id !== id)
			setDevices([...d])
			state.renderPage("devices")
		}
	}

	const saveDevice = async () => {
		let resp = undefined
		let ok = false
		if (device._id !== undefined) {
			resp = await state.callController(null, null, "POST", "/v3/device/update", { Device: device }, false, false)
			if (resp.status === 200) {
				ok = true
			}
		} else {
			resp = await state.callController(null, null, "POST", "/v3/device/create", { Device: device }, false, false)
			if (resp.status === 200) {
				ok = true
				devices.push(resp.data)
				setDevices([...devices])
			}
		}

		return ok
	}

	const newDevice = () => {
		setDevice({ Tag: "", Groups: [] })
		setEditModalOpen(true)
	}

	useEffect(() => {
		getDevices(0, 100)
	}, [])

	let table = {
		data: devices,
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		columns: {
			Tag: true,
			_id: true,
			CreatedAt: true,
		},
		columnFormat: {
			CreatedAt: (obj) => {
				return dayjs(obj.CreatedAt).format("HH:mm:ss DD-MM-YYYY")
			}
		},
		Btn: {
			Edit: (obj) => {
				setDevice(obj)
				setEditModalOpen(true)
			},
			Delete: (obj) => {
				deleteDevice(obj._id)
			},
			New: () => {
				newDevice()
			},
		},
		columnClass: {},
		headers: ["Tag", "ID", "CreatedAt"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
		more: getDevices,
	}

	return (
		<div className="">
			<GenericTable table={table} />

			<NewObjectEditorDialog
				open={editModalOpen}
				onOpenChange={setEditModalOpen}
				object={device}
				title="Device"
				description=""
				readOnly={false}
				saveButton={async () => {
					console.log("save")
					let ok = await saveDevice()
					if (ok === true) {
						setEditModalOpen(false)
						state.renderPage("devices")
					}
				}}
				onChange={(key, value, type) => {
					device[key] = value
				}}
				onArrayChange={(key, value, index) => {
					device[key][index] = value;
				}}
			/>

		</div >
	)
}

export default Devices;
