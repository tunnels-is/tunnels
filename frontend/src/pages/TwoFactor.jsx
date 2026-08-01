import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import QRCode from "react-qr-code"
import { Card, Page, TextField } from "@/components/ui"
import { api, controller } from "@/store/actions"
import { useStore } from "@/store/store"

const TwoFactor = () => {
	const navigate = useNavigate()
	const user = useStore((s) => s.user)
	const notifyError = useStore((s) => s.notifyError)

	const [qrValue, setQrValue] = useState("")
	const [recoveryCodes, setRecoveryCodes] = useState("")
	const [inputs, setInputs] = useState({})
	const [errors, setErrors] = useState({})

	useEffect(() => {
		if (!user) {
			navigate("/login")
			return
		}
		api("getQRCode", { Email: user.Email }).then((resp) => {
			if (resp.status === 200 && resp.data?.Value) setQrValue(resp.data.Value)
		})
	}, [])

	const confirm = async () => {
		const errs = {}
		if (!inputs.password) errs.password = "Please enter your password"
		if (!inputs.digits || inputs.digits.length !== 6) errs.digits = "Authenticator code must be 6 digits"
		setErrors(errs)
		if (Object.keys(errs).length > 0) return

		const secret = qrValue.split("&")[0]?.split("secret=")[1] || qrValue.split("&")[1]?.split("=")[1]
		if (!secret) {
			notifyError("Could not parse authenticator secret")
			return
		}

		const resp = await controller("/client/user/2fa/confirm", { ...inputs, Code: secret })
		if (resp.status === 200) setRecoveryCodes(resp.data?.Data)
	}

	return (
		<Page>
			<div className="max-w-md">
				<Card>
					{qrValue && !recoveryCodes && (
						<>
							<p className="mb-3 text-xs opacity-60">
								Scan the QR code with your authenticator app, then confirm.
							</p>
							<div className="mx-auto w-fit rounded-box bg-white p-4">
								<QRCode value={qrValue} style={{ height: "auto", width: "220px" }} viewBox="0 0 256 256" />
							</div>
							<div className="mt-4 space-y-2">
								<div>
									<TextField
										label="Password"
										type="password"
										placeholder="Your account password"
										value={inputs.password || ""}
										onChange={(e) => setInputs({ ...inputs, password: e.target.value })}
									/>
									{errors.password && <p className="text-xs text-error">{errors.password}</p>}
								</div>
								<div>
									<TextField
										label="Authenticator Code"
										placeholder="6-digit code"
										value={inputs.digits || ""}
										onChange={(e) => setInputs({ ...inputs, digits: e.target.value })}
									/>
									{errors.digits && <p className="text-xs text-error">{errors.digits}</p>}
								</div>
								<button className="btn btn-primary btn-block btn-sm mt-2" onClick={confirm}>
									Confirm
								</button>
								<div className="mt-3 border-t border-base-300 pt-3">
									<p className="mb-1 text-xs opacity-60">
										Have a recovery code? Enter it below to replace existing 2FA.
									</p>
									<TextField
										label="Recovery Code"
										placeholder="Recovery Code"
										value={inputs.recovery || ""}
										onChange={(e) => setInputs({ ...inputs, recovery: e.target.value })}
									/>
								</div>
							</div>
						</>
					)}

					{recoveryCodes && (
						<div>
							<h3 className="mb-2 font-semibold">Recovery Codes</h3>
							<div className="alert alert-error mb-3 text-xs">
								DO NOT STORE THESE CODES WITH YOUR PASSWORD
							</div>
							<code className="break-all font-mono text-sm">{recoveryCodes}</code>
						</div>
					)}
				</Card>
			</div>
		</Page>
	)
}

export default TwoFactor
