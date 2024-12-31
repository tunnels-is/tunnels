import React, { useDebugValue, useEffect, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import Loader from "react-spinners/ScaleLoader";
import STORE from "../store";
import GLOBAL_STATE, { STATE } from "../state";
import dayjs from "dayjs";
import KeyValue from "./component/keyvalue";

const Routing = () => {
	const state = GLOBAL_STATE("connections")
	const navigate = useNavigate()

	let modifiedCons = state.GetModifiedConnections()
	let user = STORE.GetUser()

	if (!user) {
		return (<Navigate to={"/login"} />)
	}

	useEffect(() => {
		let x = async () => {
			await state.GetBackendState()
			await state.GetServers()
		}
		x()
	}, [])


	const renderRoute = (c) => {

		let modified = undefined
		modifiedCons.forEach(modC => {
			if (modC.WindowsGUID == c.WindowsGUID) {
				modified = modC
				c = modC
				return
			}
		});

		let active = false
		state.State.ActiveConnections?.map((x) => {
			if (x.WindowsGUID === c.WindowsGUID) {
				active = true
				return
			}
		})

		let server = undefined
		// if (!c.Private) {
		state.Servers?.map((x) => {
			if (x._id === c.ServerID) {
				server = x
				return
			}
		})

		if (!server) {
			state.PrivateServers?.map((x) => {
				if (x._id === c.ServerID) {
					server = x
					return
				}
			})
		}

		// }

		let s = undefined
		state?.State?.ConnectionStats?.map(cs => {
			if (cs.StatsTag === c.Tag) {
				s = cs
				return
			}
		})



		let countries = []
		state?.State?.AvailableCountries?.map(c => {
			countries.push({ value: c, key: c })
		})


		console.log("1")

		return (
			<div className={`connection-card`} key={c.WindowsGUID}>

				<div className="item enable-tooltip">
					<div className="tag" onClick={() => {
						// openConnectionEditor(c)
					}}>{c.Tag}</div>
				</div>

				{c.Private &&
					<>
						<div className="button-and-text-seperator"></div>
						<KeyValue value={server?.Tag} label={"Server"} />
						<KeyValue value={c.PrivateIP} label={"IP"} />
						<KeyValue value={c.PrivatePort} label={"Port"} />
						<KeyValue value={c.PrivateCert} label={"Cert"} />
					</>
				}

				{(server && !c.Private) &&
					<>
						<div className="button-and-text-seperator"></div>
						<KeyValue value={server.Tag} label={"Server"} />
						<KeyValue value={server.IP} label={"IP"} />
						<KeyValue value={server.MS} label={"MS"} />
					</>
				}
				{(!c.ServerID === "" && !c.Private) &&
					<div className="item">
						<div className="card-button"
							onClick={() => {
								navigate("/servers")
							}}

						>
							Select A Server
						</div>
					</div>
				}

				<div className="button-and-text-seperator"></div>
				<KeyValue value={String(c.IFName)} label={"Interface"} />
				<KeyValue value={String(c.IPv4Address)} label={"IPv4"} />
				<KeyValue value={String(c.EnableDefaultRoute)} label={"Default Route"} />

				{s?.DHCP &&
					<>
						<div className="button-and-text-seperator"></div>
						<KeyValue value={String(s.VPLNetwork?.Network)} label={"VPL Network"} />
						<KeyValue value={String(s.DHCP?.IP).replaceAll(",", ".")} label={"IP"} />
						<KeyValue value={String(s.DHCP?.Hostname)} label={"Host"} />
					</>
				}

			</div >
		)
	}

	console.log('HERE!')

	return (
		<div className="connections" >
			{(!state.Config?.Connections || state.Config?.Connections.length < 1) &&
				<Loader
					key={"loader"}
					className="spinner"
					loading={true}
					color={"#20C997"}
					height={100}
					width={50}
				/>
			}


			{state.Config?.Connections?.map((c) => {
				return renderRoute(c)
			})}

			<div className="plus-button"
				onClick={() => {
					// addConnection()
				}}
			>
				+
			</div>

		</div>
	);
}

export default Routing;
