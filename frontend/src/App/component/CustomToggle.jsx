import React, { useEffect, useState } from "react";

const CustomToggle = (props) => {
	if (props.value === undefined) {
		return (<></>)
	}

	return (
		<div className="ab custom-toggle">
			<div className="label">
				{props?.label}
			</div>

			<div
				onClick={() => props.toggle()}
				className="slider">
				<div
					className={`nob ${String(props.value)}`}
				></div>
			</div>


		</div >
	)
}

export default CustomToggle
