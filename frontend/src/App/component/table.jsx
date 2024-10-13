import React, { useEffect, useState } from "react";

const CustomTable = (props) => {
	const [filter, setFilter] = useState("")
	const [currentPage, setCurrentPage] = useState(1)

	let pageSize = 15;
	let totalPages = Array.from({
		length: (Math.ceil(props?.rows.length / pageSize))
	}, (_, i) => i + 1)

	const onPageChange = (p) => {
		setCurrentPage(Number(p))
	}

	const currentTableData = () => {
		const first = (currentPage - 1) * pageSize;
		const last = (first + pageSize);

		return props?.rows.slice(first, last);
	}

	const onNext = () => {
		if ((currentPage + 1) <= totalPages[totalPages.length - 1]) {
			setCurrentPage(currentPage + 1);
		}
	}

	const onPrev = () => {
		if ((currentPage - 1) > 0) {
			setCurrentPage(currentPage - 1);
		}
	}

	return (
		<div className="table-frame">

			<div className="top-bar">

				{props?.title &&
					<div className="title">{props.title}</div>
				}
				<div className="search-bar">
					<input
						onChange={(e) => setFilter(e.target.value)}
						placeholder={props?.placeholder ? props.placeholder : "Search.."}
						className="ab" />
					{props?.button &&
						<div onClick={(e) => props.button.click(e)} className="text-button">
							{props.button.text}
						</div>
					}
				</div>
			</div>

			<table className={`${props.className} ab table`}>
				<tbody>

					<tr className="ab header">
						{props?.header?.map(l => {
							let cs = {}

							if (l.color) {
								// cs.color = COLORS[l.color]
								cs.color = "var(--c-" + l.color + ")"
							}

							if (l.align) {
								cs.textAlign = l.align
							}

							return (
								<th
									key={l.value}
									style={cs}
									className="ab column content">{l.value}
								</th>
							)

						})}
					</tr>

					{currentTableData().map(r => {
						if (filter !== "") {
							let shouldShow = false
							r.items.map(i => {
								if (String(i.value).includes(filter)) {
									shouldShow = true
								}
							})
							if (!shouldShow) {
								return
							}
						}

						return (
							<tr className="row">
								{r.items.map(i => {
									let cs = {}
									let clicky = function() {

									}
									if (i.click) {
										clicky = i.click
									}

									if (i.color) {
										// cs.color = COLORS[i.color]
										cs.color = "var(--c-" + i.color + ")"
									}

									if (i.align) {
										cs.textAlign = i.align
									}

									if (i.type === "text") {

										return (
											<td
												key={i.value}
												style={cs}
												onClick={(e) => clicky(e)}
												className={`ab column content ${i.click ? "clickable" : ""}`}>
												{i.value}
											</td>
										)
									} else if (i.type === "select") {

										return (
											<td
												key={i.value}
												style={cs} className="ab column">
												{i.value}
											</td>
										)
									} else if (i.type === "img") {
										return (
											<td
												key={i.value}
												style={cs} className="ab column">
												<img src={i.value} alt="x" />
											</td>
										)
									}
								})}
							</tr>
						)
					})}
				</tbody>
			</table >
			{totalPages.length > 1 &&
				<div className="pagination-bar">
					<div className="left-arrow" onClick={onPrev}>
						&lt;
					</div>
					<div className="pages">
						<select
							className="page-selection"
							value={currentPage}
							onChange={e => onPageChange(e.target.value)}>
							{totalPages.map((i) => (
								<option value={i}> {i} </option>
							))}
						</select>
					</div>
					<div className="right-arrow" onClick={onNext}>
						&gt;
					</div>
				</div>}
		</div>
	)
}

export default CustomTable
