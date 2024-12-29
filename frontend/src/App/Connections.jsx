import React, { useDebugValue, useEffect, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import Loader from "react-spinners/ScaleLoader";
import STORE from "../store";
import GLOBAL_STATE, { STATE } from "../state";
import dayjs from "dayjs";
import KeyValue from "./component/keyvalue";

const Connections = () => {
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

	const addConnection = () => {
		let new_conn = {
			Tag: "newtag",
			IFName: "newconn",
			IFIP: "0.0.0.0",
		}
		state.createConnection(new_conn).then(function(conn) {
			if (conn !== undefined) {
				state.renderPage("connections")
			}
		})
	}

	const openConnectionEditor = (con) => {
		state.editorData = con
		state.editorReadOnly = false
		state.editorDelete = function() {
			state.DeleteConnection(con.WindowsGUID)
		}
		state.editorSave = function() {
			state.ConfigSave()
		}
		state.editorOnChange = function(data) {
			// console.dir(data)
			state.editorError = ""
			let x = undefined
			try {
				x = JSON.parse(data)
			} catch (error) {
				console.log(error.message)
				console.dir(error)
				if (error.message) {
					state.editorError = error.message
				} else {
					state.editorError = "Invalid JSON"
				}
				return
			}

			let modCons = state.GetModifiedConnections()
			let found = false
			modCons.forEach((n, i) => {
				if (n.WindowsGUID === x.WindowsGUID) {
					modCons[i] = x
					found = true
				}
			})

			if (!found) {
				modCons.push(x)
			}

			state.SaveConnectionsToModifiedConfig(modCons)
			state.globalRerender()
		}

		state.editorExtraButtons = []
		state.editorExtraButtons.push({
			func: function(data) {
				console.dir(data)
				let ed = state.editorData
				console.dir(ed)
				if (ed === undefined) {
					console.log("no data!")
					return
				}
				if (ed.Networks.length < 1) {
					ed.Networks.push({
						Tag: "base-network",
						Network: "",
						Nat: "",
						Routes: []
					})
				}
				ed.Networks[0].Routes.push(
					{
						Address: "0.0.0.0/32",
						Metric: "0"
					}
				)

				// state.SaveConnectionsToModifiedConfig(modCons)
				state.globalRerender()
			},
			title: "+Route"
		})
		state.editorExtraButtons.push({
			func: function(data) {
				console.dir(data)
			},
			title: "+Network"
		})


		state.globalRerender()

	}


	const renderConnection = (c) => {

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


		let ib = 0
		let eb = 0
		let ibs = "B/s"
		let ebs = "B/s"
		let stocms = "?"
		let roundms = "?"

		if (s) {
			stocms = String(s.ServerToClientMicro / 1000)
			roundms = stocms * 2

			eb = s.EgressBytes
			ib = s.IngressBytes
			if (s.EgressBytes > 1000) {
				if (s.EgressBytes > 1000000) {
					ebs = "MB/s"
					eb = s.EgressBytes / 1000000
				} else {
					ebs = "KB/s"
					eb = s.EgressBytes / 1000
				}
			}

			if (s.IngressBytes > 1000) {
				if (s.IngressBytes > 1000000) {
					ibs = "MB/s"
					ib = s.IngressBytes / 1000000
				} else {
					ibs = "KB/s"
					ib = s.IngressBytes / 1000
				}
			}
		}

		return (
			<div className={`connection-card`} key={c.WindowsGUID}>

				<div className="item enable-tooltip">
					<div className="tag" onClick={() => {
						openConnectionEditor(c)
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
				<KeyValue value={String(c.MTU)} label={"MTU"} />
				<KeyValue value={String(c.TxQueueLen)} label={"TX Queue"} />
				<KeyValue value={String(c.IPv4Address)} label={"IPv4"} />
				<KeyValue value={String(c.IPv6Address)} label={"IPv6"} />
				<KeyValue value={String(c.NetMask)} label={"Net Mask"} />
				<div className="button-and-text-seperator"></div>
				<KeyValue value={String(c.EnableDefaultRoute)} label={"Default Route"} />
				<KeyValue value={STORE.EncryptionTypes[c.EncryptionType]} label={"Encryption Type"} />
				<KeyValue value={String(c.AutoReconnect)} label={"Auto Reconnect"} />
				<KeyValue value={String(c.Persistent)} label={"Persistent"} />
				<KeyValue value={String(c.PreventIPv6)} label={"Prevent IPv6"} />
				<div className="button-and-text-seperator"></div>
				<KeyValue value={String(c.DNSBlocking)} label={"DNS blocking"} />
				<KeyValue value={String((c.DNSServers && c.DNSServers.length > 0) ? c.DNSServers[0] : "Default")} label={"Primary DNS"} />
				<KeyValue value={String((c.DNSServers && c.DNSServers.length > 1) ? c.DNSServers[1] : "")} label={"Secondary DNS"} />

				{s &&
					<>
						{s.DHCP &&
							<>
								<div className="button-and-text-seperator"></div>
								<KeyValue value={String(s.VPLNetwork?.Network)} label={"VPL Network"} />
								<KeyValue value={String(s.DHCP?.IP).replaceAll(",", ".")} label={"IP"} />
								<KeyValue value={String(s.DHCP?.Hostname)} label={"Host"} />
							</>
						}
						<div className="button-and-text-seperator"></div>
						<KeyValue label={"Up Nonce"} value={s.Nonce1} />
						<KeyValue label={"Upload"} value={s.EgressString} />
						<KeyValue label={"Down Nonce"} value={s.Nonce2} />
						<KeyValue label={"Download"} value={s.IngressString} />
						<div className="button-and-text-seperator"></div>
						<KeyValue label={"Last Ping"} value={dayjs(s.PingTime).format('HH:mm:ss')} />
						<KeyValue label={"Server to Client"} value={stocms + " ms"} />
						<KeyValue label={"Round Trip"} value={roundms + " ms"} />
						<KeyValue label={"Memory"} value={s.MEM + " %"} />
						<KeyValue label={"Disk"} value={s.DISK + " %"} />
						<KeyValue label={"CPU"} value={s.CPU + " %"} />
					</>
				}

				{!active &&
					<div className="item enable-tooltip">
						<div className="card-button connect"
							onClick={() => state.ConfirmAndExecute(
								"success",
								"connect",
								10000,
								"",
								"Connect to " + c.Tag,
								() => {
									state.connectToVPN(c)
								})}
						>CONNECT</div>
					</div>
				}
				{
					active &&
					<div className="item enable-tooltip">
						<div className="card-button danger-button"
							onClick={() => state.ConfirmAndExecute(
								"success",
								"disconnect",
								10000,
								"",
								"Disconnect from " + c.Tag,
								() => {
									state.disconnectFromVPN(c)
								})}
						>DISCONNECT</div>
					</div>
				}
			</div >
		)
	}


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

			<div className="add-connection"
				onClick={() =>
					addConnection()
				}
			>
				New Tunnel
			</div>

			{state.Config?.Connections?.map((c) => {
				return renderConnection(c)
			})}

		</div>
	);
}

export default Connections;
