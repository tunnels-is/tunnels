import React, { useEffect } from "react";

const ToggleKeyValue = (props) => {
	if (props?.value === undefined) {
		return (<></>)
	}

	return (
		<div className="ab togglekv">
			{props.value === true &&
				<div onClick={props.onClick} className="label enabled">
					Enabled
				</div>
			}

			{props.value === false &&
				<div onClick={props.onClick} className="label disabled">
					Disabled
				</div>
			}

			<div className="value">
				{props?.label}
			</div>

		</div >
	)
}

export default ToggleKeyValue
