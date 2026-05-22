import React, { useEffect, useState } from "react";
import { v4 as uuidv4 } from "uuid";
import GLOBAL_STATE from "../state";
import STORE from "../store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select.jsx";
import { useNavigate, useParams } from "react-router-dom";
import { CopyPlusIcon, Edit2Icon, Save } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

const useForm = () => {
  const { modeParam } = useParams()
  let mm = 0
  if (!modeParam || modeParam === 0) {
    mm = 1
  } else {
    mm = modeParam
  }
  const [inputs, setInputs] = useState({});
  const [tokenLogin, setTokenLogin] = useState(false);
  const [errors, setErrors] = useState({});
  const [mode, setMode] = useState(Number(mm));
  const [remember, setRememeber] = useState(false);
  const state = GLOBAL_STATE("login");
  const [authServer, setAuthServer] = useState()
  const navigate = useNavigate()
  const [newAuth, setNewAuth] = useState({
    ID: uuidv4(),
    Host: "",
    Port: "",
    HTTPS: true,
    ValidateCertificate: true,
    CertificatePath: "",
  })
  const [modalOpen, setModalOpen] = useState(false)

  const changeAuthServer = (id) => {
    console.log("changing auth servers to:", id)
    state.Config?.ControlServers?.forEach(s => {
      if (s.ID === id) {
        console.log("new auth server:", id)
        setAuthServer(s)
      }
    })

  }

  const RemoveToken = () => {
    setTokenLogin(false);
    errors["email"] = "";
    setErrors({ ...errors });
    setInputs((inputs) => ({ ...inputs, ["email"]: "" }));
  };

  const GenerateToken = () => {
    let token = uuidv4();
    setTokenLogin(true);

    setErrors({ ...errors });
    setInputs((inputs) => ({ ...inputs, ["email"]: token }));
  };

  const saveNewAuth = () => {
    let found = false
    state.Config?.ControlServers?.forEach((s, i) => {
      if (s.ID === newAuth.ID) {
        state.Config.ControlServers[i] = { ...newAuth }
        found = true
      }
    })

    if (!found) {
      state.Config.ControlServers.push(newAuth)
    }

    state.v2_ConfigSave()
  }

  const RegisterSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (inputs["email"]) {
      if (inputs["email"].length > 320) {
        errors["email"] = "Maximum 320 characters";
        hasErrors = true;
      }

      if (!tokenLogin) {
        if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
          errors["email"] = "Invalid email format";
          hasErrors = true;
        }
      }
    }

    if (!inputs["password"]) {
      errors["password"] = "Password missing";
      hasErrors = true;
    }
    if (!inputs["password2"]) {
      errors["password2"] = "Password confirm missing";
      hasErrors = true;
    }

    if (inputs["password"] !== inputs["password2"]) {
      errors["password2"] = "Passwords do not match";
      hasErrors = true;
    }

    if (inputs["password"]) {
      if (inputs["password"].length < 10) {
        errors["password"] = "Minimum 10 characters";
        hasErrors = true;
      }
      if (inputs["password"].length > 255) {
        errors["password"] = "Maximum 255 characters";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }


    let x = await state.callController(authServer, "POST", "/client/user/create", inputs, true, false)
    if (x.status === 200) {
      state.v2_SetUser(x.data, remember, authServer);
      navigate("/")
      return
    }
    setErrors({});
  };

  const HandleSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"] || inputs["email"] === "") {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (!inputs["password"] || inputs["password"] === "") {
      errors["password"] = "Password missing";
      hasErrors = true;
    }

    if (mode === 1) {
      if (!inputs["devicename"] || inputs["devicename"] === "") {
        errors["devicename"] = "Device login name missing";
        hasErrors = true;
      }
    }

    if (mode === 2) {
      if (!inputs["digits"] || inputs["digits"] === "") {
        errors["digits"] = "Authenticator code missing";
        hasErrors = true;
      }

      if (inputs["digits"] && inputs["digits"].length < 6) {
        errors["digits"] = "Code needs to be at least 6 digits";
        hasErrors = true;
      }
    }

    if (mode === 3) {
      if (!inputs["recovery"] || inputs["recovery"] === "") {
        errors["recovery"] = "Recovery code missing";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let x = await state.callController(authServer, "POST", "/client/user/login", inputs, true, false)
    if (x && x.status === 200) {
      STORE.Cache.Set("default-device-name", inputs["devicename"]);
      STORE.Cache.Set("default-email", inputs["email"]);
      state.v2_SetUser(x.data, remember, authServer);
      if (mode === 3) {
        navigate("/twofactor/recover")
      } else {
        navigate("/")
      }
      return
    }
    setErrors({});
  };

  const EnableSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (inputs["email"]) {
      if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
        errors["email"] = "Email address format is incorrect";
        hasErrors = true;
      }
    }

    if (!inputs["code"]) {
      errors["code"] = "code missing";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let request = {
      Email: inputs["email"],
      ConfirmCode: inputs["code"],
    };

    let x = await state.callController(authServer, "POST", "/client/user/enable", request, true, false)
    if (x.status === 200) {
      inputs["code"] = "";
      setInputs({ ...inputs });
      setMode(6);
    }
    setErrors({});
  };

  const ResetSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (!inputs["password"]) {
      errors["password"] = "Password missing";
      hasErrors = true;
    }

    if (inputs["password"] && inputs["password"].length < 9) {
      errors["password"] =
        "Password needs to be at least 9 characters in length";
      hasErrors = true;
    }

    if (inputs["password"] && inputs["password"].length > 255) {
      errors["password"] = "Password can not be longer then 255 characters";
      hasErrors = true;
    }

    if (!inputs["password2"]) {
      errors["password2"] = "Password confirmation missing";
      hasErrors = true;
    }

    if (inputs["password"] !== inputs["password2"]) {
      errors["password"] = "Passwords do not match";
      hasErrors = true;
    }

    if (!inputs["code"]) {
      errors["code"] = "code missing";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let request = {
      Email: inputs["email"],
      Password: inputs["password"],
      ResetCode: inputs["code"],
      UseTwoFactor: inputs["usetwofactor"] ? inputs["usetwofactor"] : false
    };

    let x = await state.callController(authServer, "POST", "/client/user/reset/password", request, true, false)
    if (x.status === 200) {
      inputs["password"] = "";
      inputs["password2"] = "";
      inputs["code"] = "";
      setInputs({ ...inputs });
      setMode(1);
    }
    setErrors({});
  };

  const GetCode = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email missing";
      hasErrors = true;
    }

    if (inputs["email"]) {
      if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
        errors["email"] = "Email address format is incorrect";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let request = {
      Email: inputs["email"],
    };

    let x = await state.callController(authServer, "POST", "/client/user/reset/code", request, true, false)
    if (x.status === 200) {
      state.successNotification("reset code sent")
    }
    setErrors({});
  };

  const HandleInputChange = (event) => {
    setInputs((inputs) => ({
      ...inputs,
      [event.target.name]: event.target.value,
    }));
  };

  return {
    state,
    remember,
    setRememeber,
    inputs,
    setInputs,
    HandleInputChange,
    HandleSubmit,
    errors,
    setMode,
    mode,
    RegisterSubmit,
    GenerateToken,
    tokenLogin,
    ResetSubmit,
    GetCode,
    RemoveToken,
    EnableSubmit,
    authServer,
    modalOpen,
    setModalOpen,
    newAuth,
    setNewAuth,
    saveNewAuth,
    changeAuthServer
  };
};

const Field = ({ label, name, type = "text", placeholder, value, error, onChange }) => (
  <div>
    <label className="label">{label}</label>
    <Input
      className="h-9 text-[13px] border-[#e7e3d7] bg-white"
      type={type} placeholder={placeholder}
      value={value || ""} name={name}
      onChange={onChange}
    />
    {error && <p className="text-[11px] text-[#dc2626] mt-1">{error}</p>}
  </div>
);

const Login = (props) => {
  const {
    state, remember, setRememeber, inputs, setInputs,
    HandleInputChange, HandleSubmit, errors, setMode, mode,
    RegisterSubmit, GenerateToken, tokenLogin, ResetSubmit,
    GetCode, RemoveToken, EnableSubmit,
    authServer, modalOpen, setModalOpen, newAuth, setNewAuth,
    saveNewAuth, changeAuthServer
  } = useForm(props);

  const GetDefaults = async () => {
    await state.GetBackendState();
    changeAuthServer(state.Config?.ControlServers[0]?.ID)
    let i = { ...inputs };
    let defaultDeviceName = STORE.Cache.Get("default-device-name");
    if (defaultDeviceName) i["devicename"] = defaultDeviceName;
    let defaultEmail = STORE.Cache.Get("default-email");
    if (defaultEmail) i["email"] = defaultEmail;
    setInputs(i);
  };

  useEffect(() => { GetDefaults(); }, []);

  const modes = [
    { value: 1, label: "Login" },
    { value: 2, label: "Register" },
    { value: 5, label: "Anonymous" },
    { value: 4, label: "Reset" },
    { value: 3, label: "2FA Recovery" },
    { value: 6, label: "Enable" },
  ];

  const showEmail = [1, 2, 4, 6].includes(mode) && !tokenLogin;
  const showToken = mode === 5 || (mode === 2 && tokenLogin);
  const showDevice = mode === 1;
  const showPassword = [1, 2, 3, 4, 5].includes(mode);
  const showConfirmPassword = [2, 4, 5].includes(mode);
  const showTwoFactor = mode === 1;
  const showRecovery = mode === 3;
  const showCode = mode === 6;
  const showResetCode = mode === 4;

  const handleSubmit = () => {
    if (mode === 1 || mode === 3) HandleSubmit();
    else if (mode === 2 || mode === 5) RegisterSubmit();
    else if (mode === 4) ResetSubmit();
    else if (mode === 6) EnableSubmit();
  };

  const submitLabel = { 1: "Login", 2: "Register", 3: "Login", 4: "Reset Password", 5: "Register", 6: "Enable Account" }[mode];

  let serverOpts = [];
  let tunID = "";
  state.Config?.ControlServers?.forEach(s => {
    if (s.Host.includes("api.tunnels.is")) tunID = s.ID;
    serverOpts.push({ value: s.ID, label: s.Host + ":" + s.Port });
  });

  return (
    <div className="w-full max-w-md mx-auto mt-[80px]">

      <div className="mb-8">
        <div className="flex items-center gap-1 mb-2">
          <span className="text-[15px] font-semibold tracking-tight text-[#0a0a0a]">Tunnels</span>
          <span className="w-1 h-1 rounded-full bg-[#0a0a0a]" />
        </div>
        <p className="text-[13px] text-[#525252]">Sign in to your account</p>
      </div>

      {/* ── Auth server dialog ── */}
      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogContent className="sm:max-w-[480px] text-[#0a0a0a] bg-white border-[#e7e3d7]">
          {newAuth && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-semibold tracking-tight text-[#0a0a0a]">Auth Server</DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="label">Host</label>
                    <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={newAuth.Host || ""} onChange={(e) => setNewAuth({ ...newAuth, Host: e.target.value })} />
                  </div>
                  <div>
                    <label className="label">Port</label>
                    <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={newAuth.Port || ""} onChange={(e) => setNewAuth({ ...newAuth, Port: e.target.value })} />
                  </div>
                </div>
                <div>
                  <label className="label">Certificate Path</label>
                  <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={newAuth.CertificatePath || ""} onChange={(e) => setNewAuth({ ...newAuth, CertificatePath: e.target.value })} />
                </div>
                <div className="flex items-center gap-2">
                  {[
                    { key: "HTTPS", label: "HTTPS" },
                    { key: "ValidateCertificate", label: "Validate Cert" },
                  ].map((opt) => (
                    <button
                      key={opt.key}
                      className={`${newAuth[opt.key]
                          ? "pill pill-active"
                          : "pill"
                        }`}
                      onClick={() => setNewAuth({ ...newAuth, [opt.key]: !newAuth[opt.key] })}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="btn btn-primary btn-md" onClick={() => { saveNewAuth(); setModalOpen(false); }}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="btn btn-ghost btn-sm" onClick={() => setModalOpen(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Form ── */}
      <div className="space-y-3 mb-6">

        {mode === 5 && (
          <div className="alert alert-danger">
            Save your login token in a secure place. It is the only form of authentication for your account. If you lose the token your account is lost forever.
          </div>
        )}

        {showToken && (
          <>
            <Field label="Token" name="email" placeholder="Token" value={inputs["email"]} error={errors["email"]} onChange={HandleInputChange} />
            {inputs["email"] && (
              <div className="alert alert-warning">
                Save this token!
              </div>
            )}
          </>
        )}

        {showEmail && <Field label="Email" name="email" type="email" placeholder="Email" value={inputs["email"]} error={errors["email"]} onChange={HandleInputChange} />}
        {showDevice && <Field label="Device Name" name="devicename" placeholder="Device Name" value={inputs["devicename"]} error={errors["devicename"]} onChange={HandleInputChange} />}
        {showPassword && <Field label="Password" name="password" type="password" placeholder="Password" value={inputs["password"]} error={errors["password"]} onChange={HandleInputChange} />}
        {showConfirmPassword && <Field label="Confirm Password" name="password2" type="password" placeholder="Confirm Password" value={inputs["password2"]} error={errors["password2"]} onChange={HandleInputChange} />}
        {showTwoFactor && <Field label="2FA Code" name="digits" placeholder="Authenticator Code (if enabled)" value={inputs["digits"]} error={errors["digits"]} onChange={HandleInputChange} />}
        {showRecovery && <Field label="Recovery Code" name="recovery" placeholder="Two Factor Recovery Code" value={inputs["recovery"]} error={errors["recovery"]} onChange={HandleInputChange} />}
        {showCode && <Field label="Code" name="code" placeholder="Confirmation Code" value={inputs["code"]} error={errors["code"]} onChange={HandleInputChange} />}
        {showResetCode && <Field label="Reset Code" name="code" placeholder="Reset Code" value={inputs["code"]} error={errors["code"]} onChange={HandleInputChange} />}

        {/* ── Actions ── */}
        <div className="flex items-center gap-3 pt-2">
          <Button
            className="btn btn-primary btn-lg"
            onClick={handleSubmit}
          >
            {submitLabel}
          </Button>

          {mode === 4 && (
            <button
              className="btn btn-link"
              onClick={GetCode}
            >
              Send Reset Code
            </button>
          )}

          {mode === 1 && (
            <button
              className={`${remember
                  ? "pill pill-active"
                  : "pill"
                }`}
              onClick={() => setRememeber(!remember)}
            >
              Remember
            </button>
          )}
        </div>
      </div>

      {/* ── Server banner ── */}
      <div className="flex items-center gap-3 py-3 px-4 rounded-lg bg-[#f4f1e8] border border-[#e7e3d7] mb-5 card-shadow">
        <span className="label shrink-0 !mb-0">Server</span>
        <Select value={authServer ? authServer.ID : tunID} onValueChange={changeAuthServer}>
          <SelectTrigger className="h-8 text-[12px] border-[#e7e3d7] bg-white flex-1">
            <SelectValue placeholder="Select Auth Server" />
          </SelectTrigger>
          <SelectContent className="bg-white border-[#e7e3d7]">
            {serverOpts.map(t => (
              <SelectItem key={t.value} value={t.value} className="text-[12px]">{t.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <button
          className="btn-icon btn-icon-md"
          onClick={() => setModalOpen(true)}
        >
          <CopyPlusIcon className="h-3.5 w-3.5" />
        </button>
        <button
          className="btn-icon btn-icon-md"
          onClick={() => { setNewAuth(authServer); setModalOpen(true); }}
        >
          <Edit2Icon className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* ── Mode pills ── */}
      <div className="flex items-center gap-1.5 flex-wrap">
        {modes.map(m => (
          <button
            key={m.value}
            className={`${mode === m.value
                ? "pill pill-active"
                : "pill"
              }`}
            onClick={() => {
              if (m.value === 5) GenerateToken();
              else if (tokenLogin) RemoveToken();
              setMode(m.value);
            }}
          >
            {m.label}
          </button>
        ))}
      </div>
    </div>
  );
};

export default Login;
