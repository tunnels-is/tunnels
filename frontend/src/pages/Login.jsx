import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Pencil, Plus } from "lucide-react"
import Dialog from "@/components/Dialog"
import { TextField, Toggle } from "@/components/ui"
import { controller, fetchState, loginUser, saveConfig } from "@/store/actions"
import { v4 as uuid } from "uuid"
import { session } from "@/store/session"
import { useStore } from "@/store/store"

const MODES = [
	{ value: 1, label: "Login" },
	{ value: 2, label: "Register" },
	{ value: 5, label: "Anonymous" },
	{ value: 4, label: "Reset" },
	{ value: 3, label: "2FA Recovery" },
	{ value: 6, label: "Enable" },
]

const SUBMIT_LABEL = { 1: "Login", 2: "Register", 3: "Login", 4: "Reset Password", 5: "Register", 6: "Enable Account" }

const emptyAuthServer = () => ({
	ID: uuid(),
	Host: "",
	Port: "",
	ValidateCertificate: true,
	CertificatePath: "",
})

const Login = () => {
	const navigate = useNavigate()
	const { modeParam } = useParams()
	const config = useStore((s) => s.config)
	const notifySuccess = useStore((s) => s.notifySuccess)
	const notifyError = useStore((s) => s.notifyError)

	const [mode, setMode] = useState(Number(modeParam) || 1)
	const [inputs, setInputs] = useState({
		email: session.get("default-email") || "",
		devicename: session.get("default-device-name") || "",
	})
	const [errors, setErrors] = useState({})
	const [remember, setRemember] = useState(false)
	const [tokenLogin, setTokenLogin] = useState(false)
	const [authServer, setAuthServer] = useState()
	const [editServer, setEditServer] = useState(null)

	useEffect(() => {
		fetchState()
	}, [])

	const servers = config?.ControlServers || []
	const activeServer = authServer || servers[0]

	const setInput = (name) => (e) => setInputs({ ...inputs, [name]: e.target.value })

	const validate = (rules) => {
		const errs = {}
		rules.forEach(([field, message, bad]) => {
			if (bad) errs[field] = message
		})
		setErrors(errs)
		return Object.keys(errs).length === 0
	}

	const validEmail = (v) => v?.includes("@") && v?.includes(".")

	const loginSubmit = async () => {
		const ok = validate([
			["email", "Email / Token missing", !inputs.email],
			["password", "Password missing", !inputs.password],
			["devicename", "Device login name missing", mode === 1 && !inputs.devicename],
			["recovery", "Recovery code missing", mode === 3 && !inputs.recovery],
		])
		if (!ok) return

		const resp = await controller("/client/user/login", { ...inputs }, { server: activeServer, auth: false })
		if (resp.status === 200) {
			session.set("default-device-name", inputs.devicename || "")
			session.set("default-email", inputs.email || "")
			await loginUser(resp.data, remember, activeServer)
			navigate("/")
		}
	}

	const registerSubmit = async () => {
		const ok = validate([
			["email", "Email / Token missing", !inputs.email],
			["email", "Maximum 320 characters", inputs.email?.length > 320],
			["email", "Invalid email format", !tokenLogin && mode === 2 && inputs.email && !validEmail(inputs.email)],
			["password", "Minimum 10 characters", !inputs.password || inputs.password.length < 10],
			["password", "Maximum 255 characters", inputs.password?.length > 255],
			["password2", "Passwords do not match", inputs.password !== inputs.password2],
		])
		if (!ok) return

		const resp = await controller("/client/user/create", { ...inputs }, { server: activeServer, auth: false })
		if (resp.status === 200) {
			await loginUser(resp.data, remember, activeServer)
			navigate("/")
		}
	}

	const enableSubmit = async () => {
		const ok = validate([
			["email", "Invalid email format", !validEmail(inputs.email)],
			["code", "Code missing", !inputs.code],
		])
		if (!ok) return

		const resp = await controller(
			"/client/user/enable",
			{ Email: inputs.email, ConfirmCode: inputs.code },
			{ server: activeServer, auth: false },
		)
		if (resp.status === 200) {
			setInputs({ ...inputs, code: "" })
			notifySuccess("Account enabled, you can now log in")
			setMode(1)
		}
	}

	const resetSubmit = async () => {
		const ok = validate([
			["email", "Email / Token missing", !inputs.email],
			["password", "Minimum 9 characters", !inputs.password || inputs.password.length < 9],
			["password", "Maximum 255 characters", inputs.password?.length > 255],
			["password2", "Passwords do not match", inputs.password !== inputs.password2],
			["code", "Reset code missing", !inputs.code],
		])
		if (!ok) return

		const resp = await controller(
			"/client/user/reset/password",
			{ Email: inputs.email, Password: inputs.password, ResetCode: inputs.code, UseTwoFactor: false },
			{ server: activeServer, auth: false },
		)
		if (resp.status === 200) {
			setInputs({ ...inputs, password: "", password2: "", code: "" })
			setMode(1)
		}
	}

	const sendResetCode = async () => {
		if (!validate([["email", "Invalid email format", !validEmail(inputs.email)]])) return
		const resp = await controller("/client/user/reset/code", { Email: inputs.email }, { server: activeServer, auth: false })
		if (resp.status === 200) notifySuccess("Reset code sent")
	}

	const submit = () => {
		setErrors({})
		if (mode === 1 || mode === 3) loginSubmit()
		else if (mode === 2 || mode === 5) registerSubmit()
		else if (mode === 4) resetSubmit()
		else if (mode === 6) enableSubmit()
	}

	const switchMode = (m) => {
		if (m === 5) {
			setTokenLogin(true)
			setInputs({ ...inputs, email: uuid() })
		} else if (tokenLogin) {
			setTokenLogin(false)
			setInputs({ ...inputs, email: "" })
		}
		setErrors({})
		setMode(m)
	}

	const saveAuthServer = async () => {

		if (!config) {
			notifyError("Configuration is still loading, please try again")
			return
		}
		const list = [...servers]
		const idx = list.findIndex((s) => s.ID === editServer.ID)
		if (idx >= 0) list[idx] = { ...editServer }
		else list.push(editServer)
		const ok = await saveConfig({ ...config, ControlServers: list })
		if (ok) setAuthServer(editServer)
		setEditServer(null)
	}

	const showToken = mode === 5 || (mode === 2 && tokenLogin)
	const showEmail = [1, 2, 4, 6].includes(mode) && !showToken
	const showPassword = [1, 2, 3, 4, 5].includes(mode)
	const showConfirmPassword = [2, 4, 5].includes(mode)

	const field = (label, name, type = "text", placeholder = label) => (
		<div>
			<TextField
				label={label}
				type={type}
				placeholder={placeholder}
				value={inputs[name] || ""}
				onChange={setInput(name)}
			/>
			{errors[name] && <p className="mt-1 text-xs text-error">{errors[name]}</p>}
		</div>
	)

	return (
		<div className="min-h-screen bg-base-200 pl-14">
			<div className="w-full max-w-md px-6 pt-20">
				<div className="mb-8">
					<div className="mb-1 flex items-center gap-1">
						<span className="text-[15px] font-semibold tracking-tight">Tunnels</span>
						<span className="h-1 w-1 rounded-full bg-primary" />
					</div>
					<p className="text-[13px] opacity-60">Sign in to your account</p>
				</div>

				<div className="mb-6 space-y-3">
					{mode === 5 && (
						<div className="alert alert-error text-sm">
							Save your login token in a secure place. It is the only form of authentication for your
							account. If you lose the token your account is lost forever.
						</div>
					)}

					{showToken && (
						<>
							{field("Token", "email")}
							{inputs.email && <div className="alert alert-warning text-sm">Save this token!</div>}
						</>
					)}
					{showEmail && field("Email", "email", "email")}
					{mode === 1 && field("Device Name", "devicename")}
					{showPassword && field("Password", "password", "password")}
					{showConfirmPassword && field("Confirm Password", "password2", "password")}
					{mode === 1 && field("2FA Code", "digits", "text", "Authenticator Code (if enabled)")}
					{mode === 3 && field("Recovery Code", "recovery", "text", "Two Factor Recovery Code")}
					{mode === 6 && field("Code", "code", "text", "Confirmation Code")}
					{mode === 4 && field("Reset Code", "code")}

					<div className="flex items-center gap-3 pt-2">
						<button className="btn btn-primary" onClick={submit}>
							{SUBMIT_LABEL[mode]}
						</button>
						{mode === 4 && (
							<button className="btn btn-link btn-sm" onClick={sendResetCode}>
								Send Reset Code
							</button>
						)}
						{mode === 1 && (
							<Toggle label="Remember" checked={remember} onChange={() => setRemember(!remember)} />
						)}
					</div>
				</div>

				<div className="mb-5 flex items-center gap-2 rounded-box border border-base-300 bg-base-100 px-4 py-3">
					<span className="shrink-0 text-xs font-medium opacity-60">Server</span>
					<select
						className="select select-sm flex-1"
						value={activeServer?.ID || ""}
						onChange={(e) => setAuthServer(servers.find((s) => s.ID === e.target.value))}
					>
						{servers.map((s) => (
							<option key={s.ID} value={s.ID}>
								{s.Host}:{s.Port}
							</option>
						))}
					</select>
					<button className="btn btn-square btn-ghost btn-sm" title="Add server" onClick={() => setEditServer(emptyAuthServer())}>
						<Plus size={14} />
					</button>
					<button
						className="btn btn-square btn-ghost btn-sm"
						title="Edit server"
						onClick={() => activeServer && setEditServer({ ...activeServer })}
					>
						<Pencil size={14} />
					</button>
				</div>

				<div className="flex flex-wrap items-center gap-1.5">
					{MODES.map((m) => (
						<button
							key={m.value}
							className={"btn btn-xs " + (mode === m.value ? "btn-primary" : "btn-ghost")}
							onClick={() => switchMode(m.value)}
						>
							{m.label}
						</button>
					))}
				</div>
			</div>

			<Dialog
				open={!!editServer}
				onClose={() => setEditServer(null)}
				title="Auth Server"
				actions={
					<>
						<button className="btn btn-ghost btn-sm" onClick={() => setEditServer(null)}>
							Cancel
						</button>
						<button className="btn btn-primary btn-sm" onClick={saveAuthServer}>
							Save
						</button>
					</>
				}
			>
				{editServer && (
					<div className="space-y-2">
						<div className="grid grid-cols-2 gap-3">
							<TextField label="Host" value={editServer.Host} onChange={(e) => setEditServer({ ...editServer, Host: e.target.value })} />
							<TextField label="Port" value={editServer.Port} onChange={(e) => setEditServer({ ...editServer, Port: e.target.value })} />
						</div>
						<TextField
							label="Certificate Path"
							value={editServer.CertificatePath}
							onChange={(e) => setEditServer({ ...editServer, CertificatePath: e.target.value })}
						/>
						<Toggle
							label="Validate Certificate"
							checked={editServer.ValidateCertificate}
							onChange={() => setEditServer({ ...editServer, ValidateCertificate: !editServer.ValidateCertificate })}
						/>
						{!editServer.ValidateCertificate && (
							<p className="text-xs text-error">
								Warning: TLS certificate verification is disabled. Your login
								credentials and device token can be intercepted by a
								man-in-the-middle on the way to this server. Only turn this off
								for a trusted self-signed server on a trusted network.
							</p>
						)}
					</div>
				)}
			</Dialog>
		</div>
	)
}

export default Login
