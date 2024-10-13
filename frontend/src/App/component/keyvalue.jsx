import React, { useEffect } from "react";

const KeyValue = (props) => {
	if (!props?.value && !props.defaultValue) {
		return (<></>)
	}

	return (
		<div className={`ab keyvalue ${props.className ? props.className : ""}`}>
			<div className="label">
				{props?.label}
			</div>

			<div className="value">
				{props?.value}
			</div>
		</div >
	)
}

export default KeyValue
