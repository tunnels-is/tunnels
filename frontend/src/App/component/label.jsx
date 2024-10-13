import React, { useEffect } from "react";

const Label = (props) => {
	if (props?.value === undefined) {
		return (<></>)
	}

	return (
		<div onClick={() => props.onClick()} className={`custom-label ${props.className}`} >
			{props?.value}
		</div >
	)
}

export default Label
