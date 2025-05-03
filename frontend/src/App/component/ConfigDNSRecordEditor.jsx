import React from "react";
import GLOBAL_STATE from "../../state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { PlusCircle, Trash2, Save, Plus } from "lucide-react";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";

const ConfigDNSRecordEditor = () => {
	const state = GLOBAL_STATE("DNSRecordForm")

	const addRecord = () => {
		if (!state.Config.DNSRecords) {
			state.Config.DNSRecords = []

		}
		state.Config?.DNSRecords.push({
			Domain: "domain.local",
			IP: [""],
			TXT: [""],
			CNAME: "",
			Wildcard: true,
		})
		state.renderPage("DNSRecordForm")
	}
	const saveAll = () => {
		state.Config.DNSRecords?.forEach((r, i) => {
			r.IP?.forEach((ip, ii) => {
				if (ip === "") {
					state.Config.DNSRecords[i].IP.splice(ii, 1)
				}
			})
			r.TXT?.forEach((txt, ii) => {
				if (txt === "") {
					state.Config.DNSRecords[i].TXT.splice(ii, 1)
				}
			})
		})
		state.ConfigSave()
		state.renderPage("DNSRecordForm")
	}
	const deleteRecord = (index) => {
		if (state.Config.DNSRecords.length === 1) {
			state.Config.DNSRecords = []
			state.v2_ConfigSave()
			state.globalRerender()
		} else {
			state.Config.DNSRecords.splice(index, 1)
			state.v2_ConfigSave()
			state.globalRerender()
		}

	}

	const updateRecord = (index, subindex, key, value) => {
		console.log("update:", index, subindex, key, value)
		if (key === "IP") {

			try {
				state.Config.DNSRecords[index].IP[subindex] = value
			} catch (error) {
				console.dir(error)
			}
		} else if (key === "TXT") {
			try {
				state.Config.DNSRecords[index].TXT[subindex] = value
			} catch (error) {
				console.dir(error)
			}

		} else if (key === "Wildcard") {
			state.Config.DNSRecords[index].Wildcard = value

		} else {
			state.Config.DNSRecords[index][key] = value
		}

		state.renderPage("DNSRecordForm")
	}

	const addIP = (index) => {
		state.Config?.DNSRecords[index].IP.push("0.0.0.0")
		state.renderPage("DNSRecordForm")
	}
	const addTXT = (index) => {
		state.Config?.DNSRecords[index].TXT.push("new text record")
		state.renderPage("DNSRecordForm")
	}



	const FormField = ({ label, children }) => (
		<div className="grid gap-2 mb-4">
			<Label className="text-sm font-medium">{label}</Label>
			{children}
		</div>
	);

	return (
		<div className="w-full max-w-4xl mx-auto p-4 space-y-6">
			<div className="flex items-center justify-between mb-6">
				<h2 className="text-2xl font-bold tracking-tight text-white">DNS Records</h2>
				<Button
					onClick={addRecord}
					variant="outline"
					className="flex items-center gap-2 text-white"
				>
					<PlusCircle className="h-4 w-4" />
					<span>Add DNS Record</span>
				</Button>
			</div>

			<div className="max-w-[350px] space-y-6">
				{state.Config?.DNSRecords?.map((r, i) => {
					if (!r) return null;

					return (
						<Card key={i} className="shadow-md transition-all hover:shadow-lg">
							<CardHeader className="bg-muted/40 pb-2">
								<CardTitle className="text-lg flex items-center gap-2">
									DNS Record {i + 1}
									{r.Wildcard && <Badge variant="outline" className="ml-2">Wildcard</Badge>}
								</CardTitle>
							</CardHeader>

							<CardContent className="pt-6 space-y-4">
								<FormField label="Domain">
									<Input
										value={r.Domain}
										onChange={(e) => updateRecord(i, 0, "Domain", e.target.value)}
										placeholder="e.g. example.com"
										className="w-full"
									/>
								</FormField>

								<FormField label="CNAME">
									<Input
										value={r.CNAME}
										onChange={(e) => updateRecord(i, 0, "CNAME", e.target.value)}
										placeholder="e.g. subdomain.example.com"
										className="w-full"
									/>
								</FormField>

								<div className="space-y-3">
									<div className="flex items-center justify-between">
										<Label className="text-sm font-medium">IP Addresses</Label>
										<Button
											size="sm"
											variant="outline"
											onClick={() => addIP(i)}
											className="h-8 text-xs"
										>
											<Plus className="h-3 w-3 mr-1" /> Add IP
										</Button>
									</div>

									{r.IP?.map((ip, ii) => (
										<Input
											key={`ip-${i}-${ii}`}
											value={ip}
											onChange={(e) => updateRecord(i, ii, "IP", e.target.value)}
											placeholder="e.g. 192.168.1.1"
											className="w-full"
										/>
									))}
								</div>

								<div className="space-y-3">
									<div className="flex items-center justify-between">
										<Label className="text-sm font-medium">TXT Records</Label>
										<Button
											size="sm"
											variant="outline"
											onClick={() => addTXT(i)}
											className="h-8 text-xs"
										>
											<Plus className="h-3 w-3 mr-1" /> Add TXT
										</Button>
									</div>

									{r.TXT?.map((txt, ii) => (
										<Textarea
											key={`txt-${i}-${ii}`}
											value={txt}
											onChange={(e) => updateRecord(i, ii, "TXT", e.target.value)}
											placeholder="Enter text record"
											className="w-full min-h-[80px]"
										/>
									))}
								</div>

								<div className="flex items-center space-x-2">
									<Switch
										id={`wildcard-${i}`}
										checked={r.Wildcard}
										onCheckedChange={(checked) => updateRecord(i, 0, "Wildcard", checked)}
									/>
									<Label htmlFor={`wildcard-${i}`}>Wildcard Domain</Label>
								</div>
							</CardContent>

							<CardFooter className="flex justify-between bg-muted/20 pt-2">
								<Button
									variant="outline"
									className="flex items-center gap-2"
									onClick={saveAll}
								>
									<Save className="h-4 w-4" />
									<span>Save</span>
								</Button>

								<Button
									variant="destructive"
									className="flex items-center gap-2"
									onClick={() => deleteRecord(i)}
								>
									<Trash2 className="h-4 w-4" />
									<span>Remove</span>
								</Button>
							</CardFooter>
						</Card>
					);
				})}

				{(!state.Config?.DNSRecords || state.Config.DNSRecords.length === 0) && (
					<div className="text-center p-12 border border-dashed rounded-lg bg-muted/30">
						<p className="text-muted-foreground">No DNS records found. Add your first record to get started.</p>
					</div>
				)}
			</div>
		</div>
	);
};

export default ConfigDNSRecordEditor;
