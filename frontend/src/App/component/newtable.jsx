import React, { useState } from "react";
import GLOBAL_STATE from "../../state";
import STORE from "../../store";

const NewTable = (props) => {
	const [filter, setFilter] = useState("")
	const state = GLOBAL_STATE(props.tableID)

	const arrayFromTotalPages = (total) => {
		if (total > 0) {
			return [...Array(total).keys()]
		} else {
			return []
		}

	}

	const setPageWrap = (page, totalPages) => {
		if (page === totalPages) {
			page = totalPages - 1
		} else if (page < 0) {
			page = 0
		}

		state.setPage(props.tableID, page)

	}

	let finalRows = []
	if (filter !== "") {
		props?.rows?.forEach(r => {
			let show = false
			r.items.forEach(item => {
				console.dir(item)
				if (String(item.value).toLowerCase().includes(filter)) {
					show = true
				}
			});

			if (show === true) {
				finalRows.push(r)
			}
		})
	} else {
		finalRows = props.rows
	}

	let pg = STORE.Cache.GetObject("table_" + props.tableID)
	let showNP = true
	let showPP = true
	let originalSize = 0
	if (pg) {
		originalSize = pg.TableSize
		if (pg.TableSize === 0) {
			pg.TableSize = finalRows.length
		}
		pg.TotalPages = Math.ceil(finalRows.length / pg.TableSize)
		if (pg.TotalPages === 0) {
			pg.TotalPages = 1
		}
		if (pg.CurrentPage < 0) {
			pg.CurrentPage = 0
		} else if (pg.CurrentPage > pg.TotalPages - 1) {
			pg.CurrentPage = pg.TotalPages - 1
		}

		pg.NextPage = pg.CurrentPage + 1
		if (pg.NextPage > pg.TotalPages) {
			showNP = false
		}

		pg.PrevPage = pg.CurrentPage - 1
		if (pg.PrevPage < 0) {
			showPP = false
		}
	} else {
		pg = {
			TableSize: 20,
			CurrentPage: 0,
			NextPage: 1,
			PrevPage: -1,
		}
		pg.TotalPages = Math.ceil(finalRows / pg.TableSize)
		STORE.Cache.SetObject("table_" + props.tableID, pg)
	}


	let indexes = []
	let x = pg.CurrentPage * pg.TableSize
	let fin = x + pg.TableSize - 1
	for (var i = x; i < fin; i++) {
		if (i < finalRows.length) {
			indexes.push(i)
		} else {
			break
		}
	}

	return (
		<div className={`new-table ${props.background ? "table-bg" : ""}`}>

			<div className="top-bar">

				{props?.title &&
					<div className="title">{props.title}</div>
				}

				{props?.button &&
					<div style={{ color: "var(--c-" + props.button.color ? props.button.color : "blue" + ")" }} onClick={(e) => props.button.click(e)} className="text-button clickable">
						{props.button.text}
					</div>
				}
				{props?.button2 &&
					<div style={{ color: "var(--c-" + props.button2.color ? props.button2.color : "blue" + ")" }} onClick={(e) => props.button2.click(e)} className="text-button clickable">
						{props.button2.text}
					</div>
				}


				{(pg.TotalPages > 1 || pg.TableSize === finalRows.length) &&
					<div className="pagination-bar">
						<div className="left-arrow" onClick={() => setPageWrap(pg.PrevPage, finalRows.length)}>
							prev
						</div>
						<div className="right-arrow" onClick={() => setPageWrap(pg.NextPage, finalRows.length)}>
							next
						</div>
						<div className="pages">
							items
							<select
								className="page-selection"
								value={originalSize}
								defaultValue={originalSize}
								onChange={e => state.setPageSize(props.tableID, e.target.value)}>
								<option value={20}> 20 </option>
								<option value={50}> 50 </option>
								<option value={100}> 100 </option>
								<option value={200}> 200 </option>
								<option value={0}> All </option>
							</select>
						</div>
						<div className="pages">
							page
							<select
								className="page-selection"
								value={pg.CurrentPage}
								onChange={e => setPageWrap(e.target.value, finalRows.length)}>
								{arrayFromTotalPages(pg.TotalPages).map((i) => (
									<option value={i}> {i + 1} </option>
								))}
							</select>
						</div>
					</div>
				}

			</div>
			<div className={` top-bar`}>
				<div className="search-bar">
					<input
						onChange={(e) => setFilter(e.target.value)}
						placeholder={props?.placeholder ? props.placeholder : "Search .."}
						className="ab" />
				</div>

			</div>

			{finalRows.length < 1 &&
				<div className="waiting">
					nothing found
				</div>

			}

			<div className={`${props.className} ab table`}>

				<div className={`ab header ${finalRows.length < 1 ? "hide" : ""}`}>
					{props?.header?.map((l, i) => {
						let cs = {}

						if (l.color) {
							// cs.color = COLORS[l.color]
							cs.color = "var(--c-" + l.color + ")"
						}

						if (l.align) {
							cs.textAlign = l.align
							cs.justifyContent = l.align
							cs.display = "flex"
						}
						if (l.minWidth) {
							cs.minWidth = l.minWidth
						}

						if (l.width) {
							cs.flex = "0 1 " + String(l.width) + "%"
						}

						console.log("HS")
						console.dir(cs)
						return (
							<div
								key={l.value + i}
								style={cs}
								className="ab column">{l.value}
							</div>
						)

					})}
				</div>

				{indexes.map(ind => {
					let r = finalRows[ind]

					return (
						<div className="row" key={"row" + ind}>
							{r.items.map((i, index) => {
								let cs = {}
								let clicky = function() {

								}
								if (i.click) {
									clicky = i.click
								}

								if (i.color) {
									cs.color = "var(--c-" + i.color + ")"
								}

								if (i.align) {
									cs.textAlign = i.align
									cs.justifyContent = i.align
									cs.display = "flex"
								}

								let classNames = ""
								if (i.className !== undefined) {
									classNames = i.className
								}
								if (i.minWidth) {
									cs.minWidth = i.minWidth
								}

								if (i.width) {
									cs.flex = "0 1 " + String(i.width) + "%"
								}

								if (i.type === "text") {
									return (
										<div
											key={i.value + String(index)}
											style={cs}
											onClick={(e) => clicky(e)}
											className={`ab tooltip column ${classNames} ${i.click ? "clickable" : ""}`}>
											{i.value}
											{i.tooltip === true &&
												<span class="tooltiptext">{i.value}</span>
											}
										</div>
									)
								} else if (i.type === "select") {
									return (
										<div
											key={i.value + String(index)}
											style={cs} className={` ${classNames} ab column`}>
											{i.value}
										</div>
									)
								} else if (i.type === "img") {
									return (
										<div
											key={i.value + String(index)}
											style={cs} className={` ${classNames} ab column`}>
											<img src={i.value} alt="x" />
										</div>
									)
								}
							})}
						</div>
					)
				})}
			</div >

		</div>
	)
}

export default NewTable
