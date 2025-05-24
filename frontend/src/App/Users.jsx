import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import GenericTable from "./GenericTable";

const Users = () => {
	const [users, setUsers] = useState([])
	const state = GLOBAL_STATE("groups")

	const getUsers = async (offset, limit) => {
		let resp = await state.callController(null, null, "POST", "/v3/user/list", { Offset: offset, Limit: limit }, false, false)
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

	let table = {
		data: users,
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		columns: {
			Email: true,
			_id: (obj) => {
				// alert(obj._id)
			},
			Updated: true,
		},
		columnFormat: {
			Updated: (obj) => {
				return dayjs(obj.Updated).format("HH:mm:ss DD-MM-YYYY")
			}
		},
		columnClass: {},
		headers: ["User", "ID", "Updated"],
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
		</div >
	)
}

export default Users;
