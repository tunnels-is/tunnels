import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import KeyValue from "./component/keyvalue";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import OrganizationForm from "./forms/OrganizationForm";

const Org = () => {
	const state = GLOBAL_STATE("org")
	const navigate = useNavigate()
	const [editOrg, setEditOrg] = useState(undefined)

	const dummyOrg = {
		Name: "",
		Address: "",
		Domains: ["myorg.local"],
		ManagerID: "",
		Information: "",
		Email: "",
		Phone: "",
	}

	useEffect(() => {
		let x = async () => {
			await state.GetBackendState()
			await state.API_GetOrg()
		}

		x()
	}, [])

	const handleUpdateSubmit = async (formData) => {
		console.log('Update ORG')
		console.dir(formData)
		await state.API_UpdateOrg(formData)
		setEditOrg(undefined)
	}

	const handleCreateSubmit = async (formData) => {
		console.log('SAVE ORG')
		console.dir(formData)
		await state.API_CreateOrg(formData)
	}

	const handleCancelEdit = () => {
		setEditOrg(undefined)
	}

	const generateGroupTable = (org) => {
		let rows = []

		org?.Groups?.forEach(g => {
			let row = {}
			row.items = [
				{
					type: "text", value: g.Tag, color: "blue", click: function(e) {
						navigate("/inspect/group/" + g._id)
					},
					width:"100",
				},
				{ type: "text", value: g._id, width:"200" },
				{ type: "text", align: "right", value: g.Users ? Object.keys(g.Users).length : 0, width:"50" },
				{ type: "text", value: g.Servers ? Object.keys(g.Servers).length : 0, width:"50" },
			]
			rows.push(row)
		})

		return rows
	}

	let rows = generateGroupTable(state?.Org)
	const headers = [
		{ value: "Tag",width:"100",},
		{ value: "ID",width:"200" },
		{ value: "Users", align: "right", width:"50"  },
		{ value: "Servers", width:"50"  }
	]

	if (editOrg) {
		return (
			<div className="ab org-wrapper">
				<div className="back" onClick={() => setEditOrg(undefined)}>Back to Organization</div>
				<OrganizationForm
					organizationData={editOrg}
					onSubmit={handleUpdateSubmit}
					onCancel={handleCancelEdit}
					formTitle="Edit Organization"
				/>
			</div>
		)
	}

	return (
		<div className="ab org-wrapper">
			{!state.Org &&
				<>
					<OrganizationForm
						organizationData={dummyOrg}
						onSubmit={handleCreateSubmit}
						formTitle="You do not have an organization, create one to continue.."
						isCreate={true}
					/>
				</>
			}
			{state.Org &&
				<>
					<div className="info frame-spacing panel">
						<div className="title"
							onClick={() => {
								setEditOrg(state.Org)
							}}
						>{state?.Org?.Name}</div>
						<KeyValue label={"Address"} value={state?.Org?.Address} />
						<KeyValue label={"Email"} value={state?.Org?.Email} />
						<KeyValue label={"Phone"} value={state?.Org?.Phone} />
						<KeyValue label={"Domains"} value={state?.Org?.Domains?.join(", ")} />
						<KeyValue label={"Information"} value={state?.Org?.Information} />
						<KeyValue label={"ID"} value={state?.Org?._id} />
					</div>

					<NewTable
						background={true}
						tableID={"org-groups"}
						title={"Groups"}
						className="group-table"
						header={headers}
						rows={rows}
						placeholder={"Search.."}
						button={{
							text: "Create",
							click: function() {
								navigate("/inspect/group")
							}
						}}
					/>
				</>
			}
		</div>
	)
}

export default Org;
