import React from "react";
import { useParams } from "react-router-dom";
import GLOBAL_STATE from "../../state"

const DNSAnswers = (props) => {
	const state = GLOBAL_STATE("dns-answers")
	console.dir(state.State.DNSResolves)
	const { domain } = useParams()

	const OpenWindowURL = (baseurl, value) => {
		// let final = "https://who.is/whois/" + value
		let final = baseurl + value
		window.open(final, "_blank")
		try {
			state.ConfirmAndExecute("", "clipboardCopy", 10000, final, "Copy link to clipboard ?", () => {
				if (navigator?.clipboard) {
					navigator.clipboard.writeText(final);
				}
				runtime.ClipboardSetText(final)
			})
		} catch (e) {
			console.log(e)
		}
	}

	let answers = new Map();

	state.State?.DNSResolves[domain].Answers?.map(a => {
		// console.dir(a)
		let as = a.split("\t")
		let children = []
		as.forEach((a, index) => {
			let isLast = false
			let isFirst = false
			let classes = "column"
			let baseURL = "https://who.is/whois/"
			if (index === as.length - 1) {
				isLast = true
				classes += " bold"
			} else if (index === 0) {
				isFirst = true
				classes += " blue  cursor"
			} else {
				classes += " dimmed"
			}
			if (a === "A" || a === "AAAA") {
				classes += " green"
			} else if (a === "CNAME") {
				classes += " green"
			}
			if (isFirst) {
				let urlvalue = ""
				if (isFirst) {
					let ds = a.split(".")
					urlvalue = ds[ds.length - 3] + "." + ds[ds.length - 2]
				}

				children.push(<div className={classes} onClick={() => {
					OpenWindowURL(baseURL, urlvalue);
				}}>{a}</div>)
			} else {
				children.push(<div className={classes}>{a}</div>)
			}
		})
		let e = <div className="answer">{children}</div>
		let key = as[4] + " " + as[2] + " " + as[3] + " " + as[0]
		let x = answers.get(key)
		if (!x) {
			answers.set(key, e)
		}
		return



	})


	console.dir(answers.size)
	return (
		<div className={`ab DNSAnswers ${props.className ? props.className : ""}`}>
			{answers.size < 1 &&
				<div className="title">no records found</div>
			}
			{Array.from(answers.entries()).map(a => {
				if (a.length > 1) {
					return (a[1])
				} else {
					return (a[0])
				}
			})}
		</div >
	)
}

export default DNSAnswers
