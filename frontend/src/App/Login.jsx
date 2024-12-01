import { useNavigate } from "react-router-dom";
import React, { useEffect, useState } from "react";

import { v4 as uuidv4 } from 'uuid';
import { DesktopIcon, EnvelopeClosedIcon, FrameIcon, LockClosedIcon, Share1Icon } from "@radix-ui/react-icons";

import GLOBAL_STATE from "../state";
import STORE from "../store";

const useForm = (props) => {
	const [inputs, setInputs] = useState({})
	const [tokenLogin, setTokenLogin] = useState(false)
	const [errors, setErrors] = useState({})
	const navigate = useNavigate()
	const [mode, setMode] = useState(1)
	const state = GLOBAL_STATE("login")

	const RemoveToken = () => {
		setTokenLogin(false)
		errors["email"] = ""
		setErrors({ ...errors })
		setInputs(inputs => ({ ...inputs, ["email"]: "" }));
	}

	const GenerateToken = () => {
		let token = uuidv4()
		setTokenLogin(true)
		errors["email"] = "SAVE THIS TOKEN!"
		setErrors({ ...errors })
		setInputs(inputs => ({ ...inputs, ["email"]: token }));
	}

	const RegisterSubmit = async () => {

		let errors = {}
		let hasErrors = false

		if (!inputs["email"]) {
			errors["email"] = "Email / Token missing"
			hasErrors = true
		}

		if (inputs["email"]) {
			if (inputs["email"].length > 320) {
				errors["email"] = "Maximum 320 characters"
				hasErrors = true
			}

			if (!tokenLogin) {
				if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
					errors["email"] = "Invalid email format"
					hasErrors = true

				}
			}
		}

		if (!inputs["password"]) {
			errors["password"] = "Password missing"
			hasErrors = true
		}
		if (!inputs["password2"]) {
			errors["password2"] = "Password confirm missing"
			hasErrors = true
		}

		if (inputs["password"] !== inputs["password2"]) {
			errors["password2"] = "Passwords do not match"
			hasErrors = true
		}

		if (inputs["password"]) {
			if (inputs["password"].length < 10) {
				errors["password"] = "Minimum 10 characters"
				hasErrors = true
			}
			if (inputs["password"].length > 255) {
				errors["password"] = "Maximum 255 characters"
				hasErrors = true
			}
		}

		if (hasErrors) {
			setErrors({ ...errors })
			return
		}

		let x = await state.Register(inputs)
		if (x.status === 200) {
			STORE.Cache.Set("default-email", inputs["email"])
			inputs["password"] = ""
			inputs["password2"] = ""
			setInputs({ ...inputs })
			setMode(1)
		}
		setErrors({})
	}

	const HandleSubmit = async () => {
		let errors = {}
		let hasErrors = false

		if (!inputs["email"] || inputs["email"] === "") {
			errors["email"] = "Email / Token missing"
			hasErrors = true
		}

		if (!inputs["password"] || inputs["password"] === "") {
			errors["password"] = "Password missing"
			hasErrors = true
		}

		if (mode === 1) {
			if (!inputs["devicename"] || inputs["devicename"] === "") {
				errors["devicename"] = "Device login name missing"
				hasErrors = true
			}
		}

		if (mode === 2) {

			if (!inputs["digits"] || inputs["digits"] === "") {
				errors["digits"] = "Authenticator code missing"
				hasErrors = true
			}

			if (inputs["digits"] && inputs["digits"].length < 6) {
				errors["digits"] = "Code needs to be at least 6 digits"
				hasErrors = true
			}
		}

		if (mode === 3) {
			if (!inputs["recovery"] || inputs["recovery"] === "") {
				errors["recovery"] = "Recovery code missing"
				hasErrors = true
			}
		}

		if (hasErrors) {
			setErrors({ ...errors })
			return
		}

		await state.login(inputs)
		setErrors({})

	}
	const EnableSubmit = async () => {

		let errors = {}
		let hasErrors = false

		if (!inputs["email"]) {
			errors["email"] = "Email / Token missing"
			hasErrors = true
		}

		if (inputs["email"]) {
			if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
				errors["email"] = "Email address format is incorrect"
				hasErrors = true
			}
		}

		if (!inputs["code"]) {
			errors["code"] = "code missing"
			hasErrors = true
		}

		if (hasErrors) {
			setErrors({ ...errors })
			return
		}

		let request = {
			Email: inputs["email"],
			ConfirmCode: inputs["code"]
		}

		let x = await state.API_EnableAccount(request)
		if (x.status === 200) {
			inputs["code"] = ""
			setInputs({ ...inputs })
			setMode(6)
		}
		setErrors({})
	}

	const ResetSubmit = async () => {

		let errors = {}
		let hasErrors = false

		if (!inputs["email"]) {
			errors["email"] = "Email / Token missing"
			hasErrors = true
		}

		// if (inputs["email"]) {
		// 	if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
		// 		errors["email"] = "Email address format is incorrect"
		// 		hasErrors = true
		// 	}
		// }

		if (!inputs["password"]) {
			errors["password"] = "Password missing"
			hasErrors = true
		}

		if (inputs["password"] && inputs["password"].length < 9) {
			errors["password"] = "Password needs to be at least 9 characters in length"
			hasErrors = true
		}

		if (inputs["password"] && inputs["password"].length > 255) {
			errors["password"] = "Password can not be longer then 255 characters"
			hasErrors = true
		}

		if (!inputs["password2"]) {
			errors["password2"] = "Password confirmation missing"
			hasErrors = true
		}

		if (inputs["password"] !== inputs["password2"]) {
			errors["password"] = "Passwords do not match"
			hasErrors = true
		}


		if (!inputs["code"]) {
			errors["code"] = "code missing"
			hasErrors = true
		}

		if (hasErrors) {
			setErrors({ ...errors })
			return
		}

		let request = {
			Email: inputs["email"],
			NewPassword: inputs["password"],
			ResetCode: inputs["code"]
		}

		let x = await state.ResetPassword(request)
		if (x.status === 200) {
			inputs["password"] = ""
			inputs["password2"] = ""
			inputs["code"] = ""
			setInputs({ ...inputs })
			setMode(1)
		}
		setErrors({})
	}


	const GetCode = async () => {

		let errors = {}
		let hasErrors = false

		if (!inputs["email"]) {
			errors["email"] = "Email missing"
			hasErrors = true
		}

		if (inputs["email"]) {
			if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
				errors["email"] = "Email address format is incorrect"
				hasErrors = true

			}
		}

		if (hasErrors) {
			setErrors({ ...errors })
			return
		}

		let request = {
			Email: inputs["email"],
		}
		let status = await state.GetResetCode(request)
		if (status === true) {
			// do we want to do anything more on success ??
		}
		setErrors({})

	}


	const HandleInputChange = (event) => {
		setInputs(inputs => ({ ...inputs, [event.target.name]: event.target.value }));
	}

	const ManualInputChange = (key, value) => {
		setInputs(inputs => ({ ...inputs, [key]: value }));
	}

	return {
		inputs,
		setInputs,
		HandleInputChange,
		ManualInputChange,
		HandleSubmit,
		errors,
		navigate,
		setMode,
		mode,
		RegisterSubmit,
		GenerateToken,
		tokenLogin,
		ResetSubmit,
		GetCode,
		RemoveToken,
		state,
		EnableSubmit,
	};
}

const Login = (props) => {

	const {
		inputs,
		setInputs,
		HandleInputChange,
		ManualInputChange,
		HandleSubmit,
		errors,
		navigate,
		setMode,
		mode,
		RegisterSubmit,
		GenerateToken,
		tokenLogin,
		ResetSubmit,
		GetCode,
		RemoveToken,
		state,
		EnableSubmit,
	} = useForm(props);


	const GetDefaults = () => {
		let i = { ...inputs }

		let defaultDeviceName = STORE.Cache.Get("default-device-name")
		if (defaultDeviceName) {
			i["devicename"] = defaultDeviceName
		}

		let defaultEmail = STORE.Cache.Get("default-email")
		if (defaultEmail) {
			i["email"] = defaultEmail
		}

		setInputs(i)
	}

	useEffect(() => {
		GetDefaults()
	}, [])


	const EmailOnlyInput = () => {
		return (<div className="input">
			<EnvelopeClosedIcon className="color-ok" width={40} height={30} center></EnvelopeClosedIcon>
			<input
				className="email-input"
				autocomplete="off"
				type="email"
				placeholder={"Email"}
				value={inputs["email"]}
				name="email"
				onChange={HandleInputChange} />
			{errors["email"] !== "" && <div className="error">
				{errors["email"]}
			</div>}
		</div>)
	}

	const EmailInput = () => {
		return (<div className="input">
			<EnvelopeClosedIcon className="color-ok" width={40} height={30} center></EnvelopeClosedIcon>
			<input
				className="email-input"
				autocomplete="off"
				type="email"
				placeholder={"Email / Token"}
				value={inputs["email"]}
				name="email"
				onChange={HandleInputChange} />
			{errors["email"] !== "" && <div className="error">
				{errors["email"]}
			</div>}
		</div>)
	}

	const DeviceInput = () => {
		return (<div className="input">
			<DesktopIcon className="color-ok" width={40} height={30} center></DesktopIcon>
			<input className="device-input"
				type="text"
				placeholder={"Device Name"}
				value={inputs["devicename"]}
				name="devicename"
				onChange={HandleInputChange} />
			{errors["devicename"] && <div className="error">
				{errors["devicename"]}
			</div>}
		</div>)

	}
	const NewPasswordInput = () => {
		return (<div className="input">
			<LockClosedIcon className="color-ok" width={40} height={30} center></LockClosedIcon>
			<input className=" pass-input"
				type="password"
				placeholder={"New Password"}
				value={inputs["password"]}
				name="password"
				onChange={HandleInputChange} />
			{errors["password"] && <div className="error">
				{errors["password"]}
			</div>}
		</div>)
	}


	const PasswordInput = () => {
		return (<div className="input">
			<LockClosedIcon className="color-ok" width={40} height={30} center></LockClosedIcon>
			<input className=" pass-input"
				type="password"
				placeholder={"Password"}
				value={inputs["password"]}
				name="password"
				onChange={HandleInputChange} />
			{errors["password"] && <div className="error">
				{errors["password"]}
			</div>}
		</div>)
	}

	const TwoFactorInput = () => {
		return (<div className="input">
			<LockClosedIcon className="color-ok" width={40} height={30} center></LockClosedIcon>
			<input className=" code-input"
				type="text"
				placeholder={"Authenticator Code (optional)"}
				value={inputs["digits"]}
				name="digits"
				onChange={HandleInputChange} />
			{errors["digits"] && <div className="error">
				{errors["digits"]}
			</div>}
		</div>)
	}

	const ConfirmPasswordInput = () => {
		return (<div className="input">
			<LockClosedIcon className="color-ok" width={40} height={30} center></LockClosedIcon>
			<input className="code-input"
				type="password"
				placeholder={"Confirm Password"}
				value={inputs["password2"]}
				name="password2"
				onChange={HandleInputChange} />
			{errors["password2"] && <div className="error">
				{errors["password2"]}
			</div>}
		</div>)
	}

	const TokenInput = () => {
		return (<div className="input">
			<FrameIcon className="color-ok" width={40} height={30} center></FrameIcon>
			<input className="token-input"
				autocomplete="off"
				placeholder={"Token / Token"}
				type="text"
				value={inputs["email"]}
				name="email"
				onChange={HandleInputChange} />
			{errors["email"] && <div className="error">
				{errors["email"]}
			</div>}
		</div>)
	}


	const CodeInput = () => {
		return (<div className="input">
			<FrameIcon className="color-ok" width={40} height={30} center></FrameIcon>
			<input
				className="code-input"
				autocomplete="off"
				type="text"
				placeholder={"Code"}
				// value={inputs["email"]}
				name="code"
				onChange={HandleInputChange} />
			{errors["code"] && <div className="error">
				{errors["code"]}
			</div>}
		</div>

		)
	}

	const RecoveryInput = () => {
		return (<div className="input">
			<FrameIcon className="color-ok" width={40} height={30} center></FrameIcon>
			<input className=" recovery-input"
				type="text"
				placeholder={"Two Factor Recovery Code"}
				value={inputs["recovery"]}
				name="recovery"
				onChange={HandleInputChange} />
			{errors["recovery"] && <div className="error">
				{errors["recovery"]}
			</div>}
		</div>

		)
	}

	const LoginForm = () => {
		return (<div className="form">

			{EmailInput()}
			{DeviceInput()}
			{PasswordInput()}
			{TwoFactorInput()}

			<div className="buttons">
				<button className={`ok-button`}
					onClick={HandleSubmit}>Login
				</button>
			</div>

		</div>)
	}
	const RegisterAnonForm = () => {
		return (<div className="form">
			<div className="warning">Save your login token in a secure place, it is the only form of authentication you
				have for your account. If you loose the token your account is lost forever.
			</div>

			{TokenInput()}
			{PasswordInput()}
			{ConfirmPasswordInput()}

			<div className="buttons">

				<button className={`ok-button`}
					onClick={RegisterSubmit}
				>Register
				</button>


			</div>

		</div>)
	}


	const RegisterForm = () => {
		return (<div className="form">

			{tokenLogin && TokenInput()}

			{!tokenLogin && EmailInput()}

			{PasswordInput()}
			{ConfirmPasswordInput()}

			<div className="buttons">

				<button className={`ok-button`}
					onClick={RegisterSubmit}
				>Register
				</button>


			</div>

		</div>)
	}

	const ResetPasswordForm = () => {
		return (<div className="form">


			{EmailOnlyInput()}
			{NewPasswordInput()}
			{ConfirmPasswordInput()}
			{CodeInput()}

			<div className="buttons">
				<button className={`reset-button`}
					onClick={() => GetCode()}>Click To Get Reset Code
				</button>
				<button className={`ok-button`}
					onClick={() => ResetSubmit()}>Reset Password
				</button>
			</div>

		</div>)
	}


	const RecoverTwoFactorForm = () => {
		return (<div className="form">

			{EmailInput()}
			{PasswordInput()}
			{RecoveryInput()}

			<div className="buttons">
				<button className={`ok-button`}
					onClick={HandleSubmit}>Login
				</button>
			</div>

		</div>)
	}

	const EnableAccountForm = () => {
		return (<div className="form">

			{EmailInput()}
			{CodeInput()}

			<div className="buttons">
				<button className={`ok-button`}
					onClick={EnableSubmit}>Enable Account
				</button>
			</div>

		</div>)
	}


	return (<div className="login-wrapper">

		{mode === 1 && LoginForm()}
		{mode === 2 && RegisterForm()}
		{mode === 4 && ResetPasswordForm()}
		{mode === 3 && RecoverTwoFactorForm()}
		{mode === 5 && RegisterAnonForm()}
		{mode === 6 && EnableAccountForm()}

		<div className="login-options">

			<button className="button"
				onClick={() => {
					RemoveToken()
					setMode(2)
				}}>Register
			</button>

			<button className="button"
				onClick={() => setMode(6)}>Enable Account
			</button>

			<button className="button"
				onClick={() => {
					GenerateToken()
					setMode(5)
				}}>Register Anonymously
			</button>

			<button className="button"
				onClick={() => setMode(1)}>Login
			</button>


			<button className="button"
				onClick={() => setMode(4)}>Reset Password
			</button>

			<button className="button"
				onClick={() => setMode(3)}>2FA Recovery
			</button>

		</div>

	</div>)

}

export default Login;
