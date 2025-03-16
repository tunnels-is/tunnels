import React, { useEffect } from "react";

import GLOBAL_STATE from "../state";
import ConfigDNSRecordEditor from "./component/ConfigDNSRecordEditor";
import STORE from "../store";

const DNSRecords = () => {
	const state = GLOBAL_STATE("dns")

	let modified = STORE.Cache.GetBool("modified_Config")

	useEffect(() => {
		state.GetBackendState()
	}, [])


	return (
		<div className="dns-page">
			{modified === true &&
			<div className="save-banner">
					<div className="button"
						onClick={() => state.v2_ConfigSave()}
						>
						Save
					</div>
					<div className="notice">Your config has un-saved changes</div>
				</div>
			}

			<ConfigDNSRecordEditor />
		</div >
	)
}

export default DNSRecords;
