
import { useEffect, useState } from "react";

const CustomSelect = (props) => {
	const [filterSelect, setFilterSelect] = useState({
		open: false,
		opt: { key: "", value: "" }
	})

	const filterState = (open, opt) => {
		setFilterSelect({ open: open, opt: opt })
		props.setValue(opt)
	}

	useEffect(() => {
		if (filterSelect.opt.value === "") {
			setFilterSelect({ open: false, opt: props.defaultOption })
		}
	}, [])

	return (
		<div key={props.parentKey} className={`custom-select ${props.className}`} >

			<div className={`default `}
				onClick={() => setFilterSelect({ open: !filterSelect.open, opt: filterSelect.opt })}
				id={filterSelect.opt.value}>{filterSelect.opt.key}
			</div>

			<span className={`options ${filterSelect.open ? 'show' : 'hide'} `}>
				{props.options.map((opt) => {
					return (
						<div key={opt.key} className={`opt`} id={opt.key}
							onClick={() => filterState(false, opt)}>
							{opt.value}
						</div>
					)
				})}
			</span>

		</div >
	)
}

export default CustomSelect;
