import React, { useState } from "react";

const InteractiveTable = (props) => {
	const [filter, setFilter] = useState("")
	// console.dir(props)

	return (
		<div className={`table-frame ${props.background ? "table-bg" : ""}`}>

			<div className="top-bar">
				{props?.title &&
					<div className="title">{props.title}</div>
				}
				<div className="search-bar">
					<input
						onChange={(e) => setFilter(e.target.value)}
						placeholder={props?.placeholder ? props.placeholder : "Search.."}
						className="ab" />

				</div>

				{props?.saveButton &&
					<div onClick={(e) => props.saveButton.click(e)} className="text-button">
						{props.saveButton.text}
					</div>
				}
				{props?.newButton &&
					<div onClick={(e) => props.newButton.click(e)} className="text-button">
						{props.newButton.text}
					</div>
				}
			</div>

			<table className={`${props.className} ab table`}>

				{props?.rows?.length > 0 &&
					<tr className="ab header">
						{props?.header?.map((l) => {
							return (
								<th key={l.value} className="ab column content">{l.value}
								</th>
							)
						})}
					</tr>
				}

				{props?.rows?.map(r => {
					if (filter !== "") {
						let shouldShow = false
						r.items.map(i => {
							if (i.originalValue) {
								if (String(i.originalValue).includes(filter)) {
									shouldShow = true
								}
							} else {
								if (String(i.value).includes(filter)) {
									shouldShow = true
								}
							}
						})
						if (!shouldShow) {
							return
						}
					}

					return (
						<tr className="row">
							{r.items.map((item) => {
								return (
									<td className={`ab column content`}>
										{item.value}
									</td>
								)
							})}
						</tr>
					)
				})}

			</table >
		</div >
	)
}

export default InteractiveTable
