import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import QRCode from "react-qr-code";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EnvelopeClosedIcon, FrameIcon, LockClosedIcon } from "@radix-ui/react-icons";
import { Input } from "@/components/ui/input";
import { useAtomValue } from "jotai";
import { userAtom } from "../stores/userStore";
import { toast } from "sonner";
import { client } from "../api/client";

const useForm = (props) => {
	const [inputs, setInputs] = useState({});
	const [errors, setErrors] = useState({});
	const [code, setCode] = useState({});
	const navigate = useNavigate();
	const user = useAtomValue(userAtom);

	const HandleSubmit = async () => {

		let errors = {}
		let hasErrors = false

		if (!inputs["digits"]) {
			errors["digits"] = "Authenticator code missing"
			hasErrors = true
		} else {
			if (inputs["digits"].length < 6) {
				errors["digits"] = "Authenticator code is too short"
				hasErrors = true
			}
			if (inputs["digits"].length > 6) {
				errors["digits"] = "Authenticator code is too long"
				hasErrors = true
			}
		}

		if (!inputs["password"]) {
			errors["password"] = "Please enter your password"
			hasErrors = true
		}

		if (hasErrors) {
			setErrors({ ...errors })
			return
		}

		if (!user) {
			navigate("/login")
		}

		let c = code.Value
		let firstSplit = c.split("&")
		let secondSPlit = firstSplit[1].split("=")
		let secret = secondSPlit[1]
		if (secret === "") {
			toast.error("Could not parse authenticator secret");
			setErrors({})
			return
		}

		inputs.Code = secret

		try {
			const response = await client.post("/v3/user/2fa/confirm", inputs);
			if (response.status === 200) {
				let c = { ...code }
				c.Recovery = response.data.Data
				setCode(c)
			}
		} catch (e) {
			toast.error("Failed to confirm 2FA");
		}

		setErrors({})
	}

	const Get2FACode = async () => {
		if (!user) {
			navigate("/login")
			return;
		}

		let data = {
			Email: user.Email
		}

		try {
			const response = await client.post("/getQRCode", data);
			if (response?.data) {
				setCode(response.data)
			} else {
				toast.error("Unknown error, please try again in a moment");
			}
		} catch (e) {
			toast.error("Failed to get QR code");
		}

		setErrors({})
	}

	const HandleInputChange = (event) => {
		setInputs(inputs => ({ ...inputs, [event.target.name]: event.target.value }));
	}

	return {
		inputs,
		HandleInputChange,
		HandleSubmit,
		errors,
		code,
		Get2FACode
	};
}


const Enable2FA = (props) => {

	const { inputs, HandleInputChange, HandleSubmit, errors, code, Get2FACode } = useForm(props);

	useEffect(() => {
		Get2FACode()
	}, [])


	return (
		<div className="w-full flex flex-col items-center justify-center p-4 bg-black">
			<div className="w-full max-w-md space-y-6">

				<Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
					<CardContent className="space-y-6 p-6">
						<div className="text-center flex-col mb-2">

							{(code.Value && !code.Recovery) &&
								<>
									<div className="qr-code p-4 bg-white w-[260px] m-auto mt-8">
										<QRCode
											className="qr"
											style={{ height: "auto", maxWidth: "220px", width: "220px" }}
											value={code.Value}
											viewBox={`0 0 256 256`}
										></QRCode>
									</div>

									<div className="space-y-2 mt-10">
										<div className="relative">
											<LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
											<Input
												id="password"
												className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
												type="password"
												placeholder="Password"
												value={inputs["password"]}
												name="password"
												onChange={HandleInputChange}
											/>
										</div>
										{errors["password"] !== "" && (
											<p className="text-sm text-red-500">{errors["password"]}</p>
										)}
									</div>

									<div className="space-y-2">
										<div className="relative">
											<LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
											<Input
												id="digits"
												className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
												type="string"
												placeholder="Two Factor Code"
												value={inputs["digits"]}
												name="digits"
												onChange={HandleInputChange}
											/>
										</div>
										{errors["digits"] !== "" && (
											<p className="text-sm text-red-500">{errors["digits"]}</p>
										)}
									</div>

									<Button className="mt-2 w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={HandleSubmit}>
										Confirm
									</Button>
									<h1 className="mt-4 text-white">Insert Two-Factor Recovery code below to over-write existing Two-Factor Authentication</h1>
									<div className="space-y-2 mt-4">
										<div className="relative">
											<LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
											<Input
												id="recovery"
												className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
												type="string"
												placeholder="Recovery Code"
												value={inputs["recovery"]}
												name="recovery"
												onChange={HandleInputChange}
											/>
										</div>
									</div>
								</>
							}

							{code.Recovery &&
								<div className="flex flex-col w-full mt-5">
									<h1 className="text-white font-bold">
										RECOVERY CODES
									</h1>
									<div className="text-red-300 mt-3">DO NOT STORE THESE CODES WITH YOUR PASSWORD</div>
									<div className="mt-4">
										{code.Recovery}
									</div>
								</div>
							}


						</div>

					</CardContent>
				</Card>
			</div>
		</div >
	)
}

export default Enable2FA;
