import React, { useState } from "react";
import dayjs from "dayjs";
import NewObjectEditorDialog from "@/components/NewObjectEditorDialog";
import GenericTable from "@/components/GenericTable";
import { useDevices, useDeleteDevice, useUpdateDevice, useCreateDevice } from "../hooks/useDevices";
import { toast } from "sonner";

const Devices = () => {
	const [offset, setOffset] = useState(0);
	const [limit, setLimit] = useState(100);
	const { data: devices, refetch } = useDevices(offset, limit);
	const deleteDeviceMutation = useDeleteDevice();
	const updateDeviceMutation = useUpdateDevice();
	const createDeviceMutation = useCreateDevice();

	const [device, setDevice] = useState(undefined)
	const [editModalOpen, setEditModalOpen] = useState(false)

	const saveDevice = async () => {
		try {
			if (device._id !== undefined) {
				await updateDeviceMutation.mutateAsync(device);
				toast.success("Device updated");
			} else {
				await createDeviceMutation.mutateAsync(device);
				toast.success("Device created");
			}
			setEditModalOpen(false);
		} catch (e) {
			toast.error("Failed to save device");
		}
	}

	const newDevice = () => {
		setDevice({ Tag: "", Groups: [] })
		setEditModalOpen(true)
	}

	let table = {
		data: devices || [],
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
				deleteDeviceMutation.mutate(obj._id, {
					onSuccess: () => toast.success("Device deleted"),
					onError: () => toast.error("Failed to delete device")
				});
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
		more: () => { }, // Pagination logic to be implemented if needed
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
				saveButton={saveDevice}
				onChange={(key, value, type) => {
					setDevice(prev => ({ ...prev, [key]: value }));
				}}
				onArrayChange={(key, value, index) => {
					setDevice(prev => {
						const newArr = [...prev[key]];
						newArr[index] = value;
						return { ...prev, [key]: newArr };
					});
				}}
			/>

		</div >
	)
}

export default Devices;
