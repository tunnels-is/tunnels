import React, { useEffect, useRef } from "react";

const FormKeyInput = (props) => {
	const xref = useRef(null)

	return (
		<div className="ab formkeyvalue">
			<div className="label">
				{props?.label}
			</div>

			<input
				size={props.value?.length}
				ref={xref}
				className="value"
				onChange={props.onChange}
				type={props.type}
				value={props.value}
				onInput={() => {
					try {
						xref.current.size = xref.current.value.length
					} catch (error) {
						console.dir(error)
					}
				}}
			/>


		</div >
	)
}

export default FormKeyInput
