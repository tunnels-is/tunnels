import React, { useEffect } from "react";

const FormKeyValue = (props) => {
	if (!props?.value) {
		return (<></>)
	}

	return (
		<div className="ab formkeyvalue">
			<div className="label">
				{props?.label}
			</div>

			<div className="value">
				{props?.value}
			</div>
		</div >
	)
}

export default FormKeyValue
