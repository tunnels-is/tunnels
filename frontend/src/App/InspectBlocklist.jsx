import React, { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import FormKeyValue from "./component/formkeyvalue";
import GLOBAL_STATE from "../state"

const InspectBlocklist = () => {
	const [newBlocklist, setNewBlocklist] = useState({
		Tag: "",
		FullPath: "",
		Enabled: false,
		Count: 0,
	});

	const initialBlocklist = {
		Tag: "",
		FullPath: "",
		Enabled: false,
		Count: 0,
	};

	const state = GLOBAL_STATE("inspect-blocklist")
	const navigate = useNavigate()

	const handleChange = (e) => {
		const { name, value } = e.target;
		setNewBlocklist(prevState => ({
			...prevState,
			[name]: value
		}));
	};

	const clearState = () => {
		setNewBlocklist({ ...initialBlocklist })
	}

	const Add = () => {
		let newLists = state.Config?.AvailableBlockLists
		newLists.push(newBlocklist);
		console.log(newLists)

		state.Config.AvailableBlockLists = newLists
		state.ConfigSave().then((ok) => {
			if (ok) {
				navigate("/dns")
			} else {
				state.RemoveModifiedConfig()
				clearState()
			}
		})
	}

	return (
		<div className="ab group-wrapper">
			<div>
				<div className="title">New Blocklist</div>
				<FormKeyValue label={"Tag"} value={
					<input name="Tag" value={newBlocklist.Tag} onChange={handleChange} type="text" />}
				/>
				<FormKeyValue label={"FullPath"} value={
					<input name="FullPath" id="fullpath" value={newBlocklist.FullPath} onChange={handleChange} type="text" />}
				/>

				<div className="card-button" onClick={() => Add()}>Add</div>
			</div>
		</div>
	)
}

export default InspectBlocklist;
