import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import NewTable from "./component/newtable";

const Users = () => {
	const [users, setUsers] = useState([])
	const state = GLOBAL_STATE("groups")

	const getUsers = async () => {
		let resp = await state.DoStuff(null, null, "POST", "/v3/user/list", { Offset: 0, Limit: 1000 }, false, false)
		if (resp.status === 200) {
			setUsers(resp.data)
		}
	}

	useEffect(() => {
		getUsers()
	}, [])


	const generateUsersTable = (users) => {
		let rows = []
		users.forEach((u, i) => {
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


	let usersRows = generateUsersTable(users)
	const usersHeaders = [
		{ value: "Email" },
		{ value: "ID", minWidth: "250px" },
		{ value: "Added" },
		{ value: "" }
	]

	return (
		<div className="ab users-wrapper">
			<NewTable
				background={true}
				title={""}
				className="group-table"
				header={usersHeaders}
				rows={usersRows}
				placeholder={"Search.."}
				button={{
					text: "Add User",
					click: () => setDialog(true)
				}}
			/>
		</div >
	)
}

export default Users;
