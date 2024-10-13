import React from "react";

import {
	ActivityLogIcon,
	DesktopIcon,
	HamburgerMenuIcon,
	LinkBreak2Icon,
	MobileIcon,
	Share1Icon
} from '@radix-ui/react-icons'


const StatsSideBar = (props) => {

	if (!props.state) {
		return (
			<div className="stats-sidebar">
				<div className="title">Loading state ...</div>
			</div>
		)
	}

	return (
		<div className="stats-sidebar">

			<div className="title">
				<DesktopIcon className="icon" height={20} width={20}></DesktopIcon>
				<div className="text">
					App State
				</div>
			</div>

			<div className="stats-item">
				<div className="label">VPN List Update</div>
				<div className="value">{props.state.SecondsUntilNodeUpdate}</div>
			</div>
			<div className="stats-item">
				<div className="label">Ready To Connect</div>
				<div className="value">{props.state.ClientReady + ""}</div>
			</div>
			<div className="stats-item">
				<div className="label">Version</div>
				<div className="value">{props.state.Version + ""}</div>
			</div>
			<div className="stats-item">
				<div className="label">Launched As Admin</div>
				<div className="value">{props.state.IsAdmin + ""}</div>
			</div>
			<div className="stats-item">
				<div className="label">Config Loaded</div>
				<div className="value">{props.state.ConfigInitialized + ""}</div>
			</div>
			<div className="stats-item">
				<div className="label">Base Folder Created</div>
				<div className="value">{props.state.BaseFolderInitialized + ""}</div>
			</div>
			<div className="stats-item">
				<div className="label">Log File Created</div>
				<div className="value">{props.state.LogFileInitialized + ""}</div>
			</div>

			<div className="title">
				<Share1Icon className="icon" height={20} width={20}></Share1Icon>
				<div className="text">
					Routers
				</div>
			</div>

			{props.state?.ActiveRouter &&
				<>
					<div className="stats-item">
						<div className="label">Entry Router</div>
						<div className="value">{props.state.ActiveRouter.Tag}</div>
					</div>
					<div className="stats-item">
						<div className="label">MS / QoS</div>
						<div className="value">{props.state.ActiveRouter.MS}{" / "}{props.state.ActiveRouter.Score} </div>
					</div>
				</>
			}

			{props.state?.ActiveNode &&
				<>
					<div className="stats-item">
						<div className="label">Exit Router</div>
						<div className="value">{props.state.ActiveNode.Router.Tag}</div>
					</div>
					<div className="stats-item">
						<div className="label">MS / QoS</div>
						<div className="value">{props.state.ActiveNode.Router.MS}{" / "}{props.state.ActiveNode.Router.Score}</div>
					</div>
					<div className="stats-item">
						<div className="label">VPN</div>
						<div className="value">{props.state.ActiveNode.Tag}</div>
					</div>
					{props.state?.Connected &&
						<div className="stats-item">
							<div className="label">Duration</div>
							<div className="value">{props.state.ConnectedTimer + ""}</div>
						</div>
					}
					<div className="stats-item">
						<div className="label">Last Ping</div>
						<div className="value">{props.state.SecondsSincePingFromRouter}</div>
					</div>
				</>
			}

			<div className="title">
				<ActivityLogIcon className="icon" height={20} width={20}></ActivityLogIcon>
				<div className="text">
					Network Stats
				</div>
			</div>
			<div className="stats-item">
				<div className="label">Download</div>
				<div className="value">{props.state.DMbpsString}</div>
			</div>
			<div className="stats-item">
				<div className="label">Packets</div>
				<div className="value">{props.state.IngressPackets}</div>
			</div>
			<div className="stats-item">
				<div className="label">Upload</div>
				<div className="value">{props.state.UMbpsString}</div>
			</div>
			<div className="stats-item">
				<div className="label">Packets</div>
				<div className="value">{props.state.EgressPackets}</div>
			</div>

		</div>
	)

}

export default StatsSideBar;
