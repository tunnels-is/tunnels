import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import { ForwardToController } from "../../wailsjs/go/main/Service";

import STORE from "../store";

const useForm = (props) => {
  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({})
  const navigate = useNavigate();

  const HandleSubmit = async () => {

    let errors = {}
    let hasErrors = false

    if (!inputs["email"]) {
      errors["email"] = "Email / token missing"
      hasErrors = true
    }

    if (inputs["email"]) {
      if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
        errors["email"] = "Email address format is incorrect"
        hasErrors = true
      }
    }

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
      ResetCode: inputs["code"],
    }

    props.toggleLoading({ tag: "RESET", show: true, msg: "Changing your password, please wait a moment..." })

    let FR = {
      Path: "user/reset/password",
      Method: "POST",
      JSONData: request,
      Timeout: 20000
    }

    ForwardToController(FR).then((x) => {
      if (e.Err) {
        props.toggleError(e.Err)
      } else {
        if (e.Code === 200) {
          setErrors({})
          props.showSuccessToast("Password has been changed", undefined)
          navigate("/login");
        } else {
          setErrors({})
          props.toggleError("Unable to reset password, please try again in a few seconds..")
        }
      }

    }).catch((e) => {
      console.dir(e)
      props.toggleError("Unknown error, please try again in a moment")
    })

    props.toggleLoading({ tag: "RESET", show: false })
  }

  const GetCode = async (event) => {
    if (event) {
      event.preventDefault();
    }

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

    let FR = {
      Path: "user/reset/code",
      Method: "POST",
      JSONData: request,
      Timeout: 20000
    }

    props.toggleLoading({ tag: "RESET", show: true, msg: "Requesting reset code, please wait..." })

    ForwardToController(FR).then((x) => {
      if (e.Err) {
        props.toggleError(e.Err)
      } else {
        if (e.Code === 200) {
          props.showSuccessToast("Password reset code has been sent to your email inbox!")
        } else {
          props.toggleError("Unable to request password reset code, please try again in a few seconds..")
        }
      }

    }).catch((e) => {
      console.dir(e)
      props.toggleError("Unknown error, please try again in a moment")
    })

    props.toggleLoading({ tag: "RESET", show: false })
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
    GetCode,
  };
}

const ResetPassword = (props) => {

  const { inputs, setInputs, HandleInputChange, HandleSubmit, errors, GetCode } = useForm(props);
  const navigate = useNavigate()

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

    window.addEventListener("keypress", function (event) {
      if (event.key === "Enter") {
        HandleSubmit()
      }
    });

    GetDefaults()

  }, [])

  return (
    <div className="reset-wrapper login-form-styles">


      <div className="login-options">
        <button className="button"
          onClick={() => navigate("/login")} >Back To Login</button>

      </div>

    </div >
  )

}

export default ResetPassword;