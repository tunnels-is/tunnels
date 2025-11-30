import React, { useState } from "react";
import GenericTable from "../components/GenericTable";
import { TableCell } from "@/components/ui/table";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import NewObjectEditorDialog from "@/components/NewObjectEditorDialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { useNavigate } from "react-router-dom";
import { CircleArrowRight, LogOut, Server } from "lucide-react";
import { useServers, useCreateServer, useUpdateServer } from "../hooks/useServers";
import { useTunnels, useConnectTunnel, useDisconnectTunnel, useUpdateTunnel } from "../hooks/useTunnels";
import { useAtomValue } from "jotai";
import { userAtom } from "@/stores/userStore";
import { activeTunnelsAtom } from "@/stores/tunnelStore";
import { getCountryName } from "@/lib/constants";
import { toast } from "sonner";

const PrivateServers = () => {
	const user = useAtomValue(userAtom);
	const activeTunnels = useAtomValue(activeTunnelsAtom);
	const { data: servers, isLoading: serversLoading } = useServers(user?.ControlServer);
	const { data: tunnels, isLoading: tunnelsLoading } = useTunnels();

	const createServerMutation = useCreateServer();
	const updateServerMutation = useUpdateServer();
	const connectTunnelMutation = useConnectTunnel();
	const disconnectTunnelMutation = useDisconnectTunnel();
	const updateTunnelMutation = useUpdateTunnel();

	const [server, setServer] = useState(undefined);
	const [editModalOpen, setEditModalOpen] = useState(false);
	const navigate = useNavigate();

	console.log(servers);
	const saveServer = () => {
		if (server._id !== undefined) {
			UpdateServer();
			return;
		}
		CreateServer();
	};

	const UpdateServer = async () => {
		updateServerMutation.mutate({ controlServer: user?.ControlServer, serverData: server }, {
			onSuccess: () => {
				setEditModalOpen(false);
			}
		});
	};

	const CreateServer = async () => {
		createServerMutation.mutate({ controlServer: user?.ControlServer, serverData: server }, {
			onSuccess: () => {
				setEditModalOpen(false);
			}
		});
	};

	const ConnectColumn = (server) => {
		let servertun = undefined;
		let assignedTunnels = 0;
		tunnels?.forEach(c => {
			if (c.ServerID === server._id) {
				servertun = c;
				assignedTunnels++;
			}
		});

		const handleConnect = () => {
			let tunnelToConnect = undefined;
			if (assignedTunnels < 1) {
				let defaultTunnel = tunnels?.find(t => t.Tag === "tunnels");
				if (defaultTunnel) {
					tunnelToConnect = defaultTunnel;
				}
			} else {
				tunnelToConnect = servertun;
			}

			if (!tunnelToConnect) {
				toast.error("No suitable tunnel found to connect");
				return;
			}

			if (!user?.DeviceToken) {
				toast.error("You are not logged in");
				return;
			}

			const connectionRequest = {
				UserID: user._id,
				DeviceToken: user.DeviceToken.DT,
				Tag: tunnelToConnect.Tag,
				EncType: tunnelToConnect.EncryptionType,
				ServerID: server._id,
				Server: user.ControlServer
			};

			connectTunnelMutation.mutate(connectionRequest);
		};

		let con = activeTunnels?.find(x => x.CR?.ServerID === server._id);

		return (
			<div>
				<DropdownMenuItem
					key="connect"
					onClick={() => {
						if (assignedTunnels > 1) {
							toast.error("Too many tunnels assigned to server");
							return;
						}
						handleConnect();
					}}
					className="cursor-pointer text-[#3a994c]"
				>
					<CircleArrowRight className="w-4 h-4 mr-2" /> Connect
				</DropdownMenuItem>
				{con && (
					<DropdownMenuItem
						key="disconnect"
						onClick={() => disconnectTunnelMutation.mutate(con.ID)}
						className="cursor-pointer text-[#ef4444]"
					>
						<LogOut className="w-4 h-4 mr-2" /> Disconnect
					</DropdownMenuItem>
				)}
			</div>
		);
	};

	const TunnelsColumn = (obj) => {
		let servertun = undefined;
		let assignedTunnels = 0;
		let opts = [];

		tunnels?.forEach(c => {
			if (c.ServerID === obj._id) {
				servertun = c;
				opts.push({ value: c.Tag, key: c.Tag, selected: true });
				assignedTunnels++;
			} else {
				opts.push({ value: c.Tag, key: c.Tag, selected: false });
			}
		});

		let value = undefined;
		let assigned = "Assign to tunnel";
		if (assignedTunnels > 1) {
			assigned = String(assignedTunnels) + " tunnels assigned";
		} else {
			value = servertun?.Tag;
		}

		return (
			<TableCell className="w-[100px] text-white">
				<Select
					value={value}
					onValueChange={(tag) => {
						const tunnel = tunnels?.find(t => t.Tag === tag);
						if (tunnel) {
							const updatedTunnel = { ...tunnel, ServerID: obj._id };
							updateTunnelMutation.mutate({ tunnel: updatedTunnel, oldTag: tunnel.Tag });
						}
					}}
				>
					<SelectTrigger className="w-full">
						<SelectValue placeholder={assigned} />
					</SelectTrigger>
					<SelectContent>
						<SelectGroup>
							{opts?.map(t => (
								<SelectItem
									key={t.value}
									value={t.value}
								>
									{t.key}
								</SelectItem>
							))}
						</SelectGroup>
					</SelectContent>
				</Select>
			</TableCell>
		);
	};

	let table = {
		data: servers || [],
		rowClick: (obj) => {
			console.log("row click!");
			console.dir(obj);
		},
		columns: {
			Tag: (obj) => {
				navigate("/server/" + obj._id);
			},
			Country: true,
			IP: true,
			Port: true,
			_id: true,
		},
		columnFormat: {
			Country: (row) => {
				return getCountryName(row.Country);
			}
		},
		columnClass: {
			Country: () => {
				return "min-w-[100px]";
			}
		},
		customColumns: {
			Tunnels: TunnelsColumn,
		},
		customBtn: {
			Connect: ConnectColumn,
		},
		Btn: {
			Edit: (obj) => {
				setServer(obj);
				setEditModalOpen(true);
			},
			Delete: (obj) => {
				// TODO: Implement delete
			},
			New: () => {
				setServer({ Tag: "", Country: "", IP: "", Port: "", DataPort: "", PubKey: "" });
				setEditModalOpen(true);
			},
		},
		headerFormat: {
			_id: () => "ID",
			Tag: () => "Name"
		},
		headers: ["Tag", "Country", "IP", "Port", "_id", "Interface"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
		more: undefined,
	};

	if (serversLoading || tunnelsLoading) {
		return <div>Loading...</div>;
	}

	return (
		<div className="w-full mt-16 p-4 space-y-6">
			<div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
				<div>
					<h1 className="text-2xl font-bold tracking-tight text-white">Private Servers</h1>
					<p className="text-muted-foreground">Manage your private VPN servers and tunnel assignments.</p>
				</div>
			</div>

			<GenericTable table={table} />

			<NewObjectEditorDialog
				open={editModalOpen}
				onOpenChange={setEditModalOpen}
				object={server}
				title="Server"
				description="Configure private server settings."
				readOnly={false}
				saveButton={() => {
					saveServer();
					setEditModalOpen(false);
				}}
				onChange={(key, value, type) => {
					setServer(prev => ({ ...prev, [key]: value }));
					console.log(key, value, type);
				}}
			/>
		</div>
	);
};

export default PrivateServers;
