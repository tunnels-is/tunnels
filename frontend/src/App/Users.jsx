import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import GenericTable from "./GenericTable";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Pencil } from "lucide-react";

const Users = () => {
	const [users, setUsers] = useState([])
	const [selectedUser, setSelectedUser] = useState(undefined)
	const [modalOpen, setModalOpen] = useState(false)
	const state = GLOBAL_STATE("groups")

	const getUsers = async (offset, limit) => {
		let resp = await state.callController(null, "POST", "/v3/user/list", { Offset: offset, Limit: limit }, false, false)
		if (resp.status === 200) {
			if (resp.data?.length === 0) {
				state.successNotification("no more users")
			} else {
				setUsers(resp.data)
			}
		}
	}

	useEffect(() => {
		getUsers(0, 50)
	}, [])

	const EditUserButton = (obj) => {
		return (
			<DropdownMenuItem
				key="edit"
				onClick={() => {
					setSelectedUser(obj)
					setModalOpen(true)
				}}
				className="cursor-pointer text-[#3a994c]"
			>
				<Pencil className="w-4 h-4 mr-2" /> Edit User
			</DropdownMenuItem>
		)
	}

	const saveUser = async (user) => {
		let resp = await state.callController(
			null,
			"POST",
			"/v3/user/adminupdate",
			{
				TargetUserID: user._id,
				Email: user.Email,
				Disabled: user.Disabled,
				IsManager: user.IsManager,
				Trial: user.Trial,
				SubExpiration: user.SubExpiration
			},
			false,
			true
		)

		if (resp === true) {
			state.successNotification("User updated successfully")
			setModalOpen(false)
			await getUsers(0, 50)
		}
	}

	let table = {
		data: users,
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		customBtn: {
			Edit: EditUserButton,
		},
		columns: {
			Email: true,
			_id: (obj) => {
				// alert(obj._id)
			},
			Trial: true,
			SubExpires: true,
			Updated: true,
		},
		columnFormat: {
			Updated: (obj) => {
				return dayjs(obj.Updated).format("HH:mm:ss DD-MM-YYYY")
			},
			SubExpires: (obj) => {
				return dayjs(obj.SubExpiration).format("HH:mm:ss DD-MM-YYYY")
			},
			Trial: (obj) => {
				return obj.Trial === true ? "Yes" : "no"
			}
		},
		columnClass: {},
		headers: ["User", "ID", "Trial", "SubExpiration", "Updated"],
		headerClass: {
			ID: () => {
				return ""
			}
		},
		opts: {
			RowPerPage: 50,
		},
		more: getUsers,
	}

	return (
		<div className="ab users-wrapper" >
			<GenericTable
				table={table}
			/>
			<NewObjectEditorDialog
				open={modalOpen}
				onOpenChange={setModalOpen}
				object={selectedUser}
				readOnly={false}
				saveButton={saveUser}
				onChange={(key, value, type) => {
					selectedUser[key] = value;
				}}
				onArrayChange={(key, value, index) => {
					selectedUser[key][index] = value;
				}}
				opts={{
					fields: {
						_id: "readonly",
						APIKey: "hidden",
						Password: "hidden",
						Password2: "hidden",
						ResetCode: "hidden",
						ConfirmCode: "hidden",
						RecoveryCodes: "hidden",
						TwoFactorCode: "hidden",
						TwoFactorEnabled: "hidden",
						Tokens: "hidden",
						IsAdmin: "hidden",
						Groups: "hidden",
						Key: "hidden",
						Updated: "readonly",
						DeviceToken: "hidden"
					},
					nameFormat: {
						IsManager: () => "Manager",
						SubExpiration: () => "Subscription Expiration"
					}
				}}
			/>
		</div >
	)
}

export default Users;
