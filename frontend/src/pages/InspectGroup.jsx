import React, { useState } from "react";
import { useParams } from "react-router-dom";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import GenericTable from "@/components/GenericTable";
import NewObjectEditorDialog from "@/components/NewObjectEditorDialog";
import { useGroup, useGroupEntities, useAddEntityToGroup, useRemoveEntityFromGroup } from "../hooks/useGroups";
import { toast } from "sonner";
import { useServers } from "../hooks/useServers";
import { useAtomValue } from "jotai";
import { userAtom } from "../stores/userStore";

const InspectGroup = () => {
	const { id } = useParams()
	const user = useAtomValue(userAtom);
	const { data: group } = useGroup(id);
	const [dialog, setDialog] = useState(false)
	const [addForm, setAddForm] = useState({})
	const [tag, setTag] = useState("user") // Default to user to match useEffect

	// Fetch entities based on current tab
	const { data: users } = useGroupEntities(id, "user", 0, 1000);
	const { data: servers } = useGroupEntities(id, "server", 0, 1000);
	const { data: devices } = useGroupEntities(id, "device", 0, 1000);

	// Fetch all servers to map IDs to Tags (if needed for display)
	const { data: allServers } = useServers(user?.ControlServer);

	const addEntityMutation = useAddEntityToGroup();
	const removeEntityMutation = useRemoveEntityFromGroup();

	const addToGroup = async () => {
		console.log("ID:", addForm.id)
		try {
			await addEntityMutation.mutateAsync({
				groupId: id,
				typeId: addForm.id,
				type: addForm.type,
				typeTag: addForm.idtype,
			});
			setDialog(false);
			toast.success("Added to group");
		} catch (e) {
			toast.error("Failed to add to group");
		}
	}

	const removeEntity = (gid, typeid, type) => {
		removeEntityMutation.mutate({ groupId: gid, typeId: typeid, type: type }, {
			onSuccess: () => toast.success("Removed from group"),
			onError: () => toast.error("Failed to remove from group")
		});
	}

	const tagChange = (tag) => {
		setDialog(false)
		setTag(tag)
	}

	const generateServerTable = () => {

		return {
			data: servers || [],
			rowClick: (obj) => {
				console.log("row click!")
				console.dir(obj)
			},
			columns: {
				Tag: (obj) => {
					// TODO
					// navigate("/inspect/server/" + obj._id)
				},
				_id: true,
			},
			columFormat: {
				Tag: (obj) => {
					const s = allServers?.find(sn => sn._id === obj._id);
					return s ? s.Tag : "??";
				},
			},
			Btn: {
				Delete: (obj) => {
					removeEntity(id, obj._id, "server")
				},
				New: () => {
					setAddForm({ id: "", type: "server", idtype: "" })
					setDialog(true)
				},
			},
			columnClass: {},
			headers: ["Tag", "ID"],
			headerClass: {},
			opts: {
				RowPerPage: 50,
			},
		}

	}


	const generateDevicesTables = () => {
		return {
			data: devices || [],
			rowClick: (obj) => {
				console.log("row click!")
				console.dir(obj)
			},
			columns: {
				Tag: (_) => {
					// TODO
					// navigate("/inspect/server/" + obj._id)
				},
				_id: true,
			},
			columFormat: {},
			Btn: {
				Delete: (obj) => {
					removeEntity(id, obj._id, "device")
				},
				New: () => {
					setAddForm({ id: "", type: "device", idtype: "" })
					setDialog(true)
				},
			},
			columnClass: {},
			headers: ["Tag", "ID"],
			headerClass: {},
			opts: {
				RowPerPage: 50,
			},
		}

	}


	let utable = {
		data: users || [],
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		columns: {
			Email: true,
			_id: true,
		},
		columFormat: {
		},
		Btn: {
			Delete: (obj) => {
				removeEntity(id, obj._id, "user")
			},
			New: () => {
				setAddForm({ id: "", type: "user", idtype: "" })
				setDialog(true)
			},
		},
		columnClass: {},
		headers: ["Username", "ID"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
	}

	if (!group) {
		return (
			<div className="ab group-wrapper">			</div>
		)
	}

	return (
		<div className="ab group-wrapper">
			<NewObjectEditorDialog
				open={dialog}
				onOpenChange={setDialog}
				object={addForm}
				opts={{
					fields: {
						idtype: "hidden",
						type: "hidden"
					},
					nameMap: {
						id: tag + " ID"
					}
				}}
				title={tag}
				readOnly={false}
				saveButton={() => {
					addToGroup()
				}}
				onChange={(key, value, type) => {
					setAddForm(prev => ({ ...prev, [key]: value }));
				}}
			/>

			<Tabs defaultValue="user" className="w-full" onValueChange={(v) => tagChange(v)}>
				<TabsList
					className="border border-[#1a1f2d] cursor-pointer rounded"
				>
					<TabsTrigger className="data-[state=active]:text-[#3168f3]" value="user">Users</TabsTrigger>
					<TabsTrigger className="data-[state=active]:text-[#3168f3]" value="server">Servers</TabsTrigger>
					<TabsTrigger className="data-[state=active]:text-[#3168f3]" value="device">Devices</TabsTrigger>
				</TabsList>
				<TabsContent value="user">
					<GenericTable table={utable} newButtonLabel={"Add"} />
				</TabsContent>
				<TabsContent className="w-full" value="server">
					<GenericTable table={generateServerTable()} newButtonLabel={"Add"} />
				</TabsContent>
				<TabsContent value="device">
					<GenericTable table={generateDevicesTables()} newButtonLabel={"Add"} />
				</TabsContent>
			</Tabs>

		</div >
	)
}

export default InspectGroup;
