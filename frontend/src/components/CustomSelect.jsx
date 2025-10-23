
import { useEffect, useState } from "react";

const CustomSelect = (props) => {
	const [filterSelect, setFilterSelect] = useState({
		open: false,
		opt: { key: "", value: "" }
	})

	const getDefaultValue = () => {
		let count = 0
		let def = ""
		props?.options?.forEach(o => {
			if (o.selected) {
				def = o.value
				count++
			}
		})
		if (count > 1) {
			return String(count) + " Assigned"
		} else {
			return def
		}
	}

	const filterState = (open, opt) => {
		setFilterSelect({ open: open, opt: opt })
		props.setValue(opt)
	}

	useEffect(() => {
		if (filterSelect.opt.value === "") {
			setFilterSelect({ open: false, opt: "" })
		}
	}, [])

	let def = getDefaultValue()
	if (def === "") {
		if (props?.placeholder) {
			def = props.placeholder
		} else {
			def = ""
		}
	}


	return (
		<div onClick={(e) => setFilterSelect({ open: !filterSelect.open, opt: "" })} key={props.parentKey} className={`new-select ${props.className}`} >

			<div className={`default`}>
				{def}
			</div>

			<div className={`opt-wrapper ${filterSelect.open ? 'show' : 'hide'}`}>
				{props.options.map((opt) => {
					return (
						<div
							key={opt.key}
							className={`opt ${opt.selected ? "active" : "inactive"}`} id={opt.key}
							onClick={() => filterState(false, opt)}>
							{opt.value}
						</div>
					)
				})}


			</div>

		</div >
	)
}

export default CustomSelect;
